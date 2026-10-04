package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	stripeapi "github.com/stripe/stripe-go/v86"
	workerapp "github.com/warmbly/warmbly/internal/app/worker"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type billingWorkerAssignment struct {
	workerapp.WorkerAssignmentService
	org    uuid.UUID
	pool   models.WarmupPoolType
	called bool
}

func (a *billingWorkerAssignment) SetOrganizationWarmupPool(_ context.Context, orgID uuid.UUID, pool models.WarmupPoolType) error {
	a.org, a.pool, a.called = orgID, pool, true
	return nil
}

func TestSubscriptionPaymentRecoveryDoesNotDemoteMailboxes(t *testing.T) {
	orgID := uuid.New()
	repo := &billingSubRepo{sub: &models.Subscription{OrganizationID: orgID}}
	assignment := &billingWorkerAssignment{}
	s := &stripeService{subRepo: repo, planRepo: &billingPlanRepo{}, workerAssignment: assignment}
	raw, err := json.Marshal(map[string]any{
		"id":       "sub_pending",
		"status":   "past_due",
		"metadata": map[string]string{"org_id": orgID.String()},
	})
	if err != nil {
		t.Fatal(err)
	}

	if xerr := s.handleSubscriptionUpdated(context.Background(), &stripeapi.Event{Data: &stripeapi.EventData{Raw: raw}}); xerr != nil {
		t.Fatal(xerr)
	}
	if assignment.called {
		t.Fatal("a non-active subscription update changed mailbox pool membership")
	}
}

type billingSubRepo struct {
	repository.SubscriptionRepository
	sub       *models.Subscription
	lookupErr error
	updated   bool
	recorded  bool
}

func (r *billingSubRepo) GetByOrganizationID(context.Context, uuid.UUID) (*models.Subscription, error) {
	return r.sub, r.lookupErr
}
func (r *billingSubRepo) GetByStripeSubscriptionID(context.Context, string) (*models.Subscription, error) {
	return nil, r.lookupErr
}
func (r *billingSubRepo) Update(_ context.Context, sub *models.Subscription) error {
	r.sub, r.updated = sub, true
	return nil
}

type billingPlanRepo struct {
	repository.PlanRepository
	plan *models.Plan
}

func (r *billingPlanRepo) GetByID(context.Context, uuid.UUID) (*models.Plan, error) {
	return r.plan, nil
}
func (r *billingPlanRepo) GetByStripePriceID(context.Context, string) (*models.Plan, error) {
	return r.plan, nil
}

func TestSubscriptionCheckoutCustomerParameters(t *testing.T) {
	for _, customerID := range []string{"", "cus_existing"} {
		t.Run("customer_"+customerID, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if r.Form.Get("mode") != "subscription" || r.Form.Has("customer_creation") || r.Form.Get("customer") != customerID {
					t.Errorf("invalid checkout parameters: %v", r.Form)
				}
				if r.Form.Get("automatic_tax[enabled]") != "false" || r.Form.Get("billing_address_collection") != "required" || r.Form.Get("tax_id_collection[enabled]") != "true" {
					t.Errorf("checkout tax parameters are wrong: %v", r.Form)
				}
				if customerID == "" && (r.Form.Has("customer_update[address]") || r.Form.Has("customer_update[name]")) {
					t.Errorf("new-customer checkout included customer_update: %v", r.Form)
				}
				if customerID != "" && (r.Form.Get("customer_update[address]") != "auto" || r.Form.Get("customer_update[name]") != "auto") {
					t.Errorf("existing-customer checkout does not save business details: %v", r.Form)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"cs_test","url":"https://checkout.stripe.com/test"}`))
			}))
			defer server.Close()
			old := stripeapi.GetBackend(stripeapi.APIBackend)
			stripeapi.SetBackend(stripeapi.APIBackend, stripeapi.GetBackendWithConfig(stripeapi.APIBackend, &stripeapi.BackendConfig{URL: stripeapi.String(server.URL), HTTPClient: server.Client()}))
			t.Cleanup(func() { stripeapi.SetBackend(stripeapi.APIBackend, old) })
			s := &stripeService{subRepo: &billingSubRepo{sub: &models.Subscription{StripeCustomerID: customerID}}}
			_, err := s.CreateCheckoutSession(context.Background(), uuid.New(), uuid.New(), "price_test", "https://example.com/success", "https://example.com/cancel", "")
			if err != nil || !called {
				t.Fatalf("checkout called=%v error=%v", called, err)
			}
		})
	}
}

func TestCreditCheckoutCollectsTaxAndBusinessDetails(t *testing.T) {
	for _, customerID := range []string{"", "cus_existing"} {
		t.Run("customer_"+customerID, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if r.Form.Get("mode") != "payment" || r.Form.Get("automatic_tax[enabled]") != "false" || r.Form.Get("billing_address_collection") != "required" || r.Form.Get("tax_id_collection[enabled]") != "true" {
					t.Errorf("invalid tax-aware credit checkout: %v", r.Form)
				}
				if customerID == "" && r.Form.Get("customer_creation") != "always" {
					t.Errorf("credit checkout will not save the new customer: %v", r.Form)
				}
				if customerID != "" && (r.Form.Get("customer") != customerID || r.Form.Get("customer_update[address]") != "auto" || r.Form.Get("customer_update[name]") != "auto") {
					t.Errorf("credit checkout does not update the existing customer: %v", r.Form)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"cs_credit","url":"https://checkout.stripe.com/test"}`))
			}))
			defer server.Close()
			old := stripeapi.GetBackend(stripeapi.APIBackend)
			stripeapi.SetBackend(stripeapi.APIBackend, stripeapi.GetBackendWithConfig(stripeapi.APIBackend, &stripeapi.BackendConfig{URL: stripeapi.String(server.URL), HTTPClient: server.Client()}))
			t.Cleanup(func() { stripeapi.SetBackend(stripeapi.APIBackend, old) })

			s := &stripeService{
				cfg:     &config.StripeConfig{CreditPackPriceIDs: map[string]string{"pack_500": "price_pack"}},
				subRepo: &billingSubRepo{sub: &models.Subscription{StripeCustomerID: customerID}},
			}
			if _, xerr := s.CreateCreditCheckoutSession(context.Background(), uuid.New(), uuid.New(), "pack_500", 500, "https://example.com/success", "https://example.com/cancel"); xerr != nil {
				t.Fatal(xerr)
			}
		})
	}
}

func TestPortalRejectsMissingCustomer(t *testing.T) {
	for _, customerID := range []string{"", " \t"} {
		url, err := (&stripeService{}).CreatePortalSession(context.Background(), customerID, "https://example.com")
		if err == nil || err.Code != errx.BadRequest || url != "" {
			t.Fatalf("url=%q error=%v", url, err)
		}
	}
}

func TestSubscriptionEventBeforeCheckout(t *testing.T) {
	orgID, planID := uuid.New(), uuid.New()
	repo := &billingSubRepo{sub: &models.Subscription{OrganizationID: orgID}}
	assignment := &billingWorkerAssignment{}
	s := &stripeService{subRepo: repo, planRepo: &billingPlanRepo{plan: &models.Plan{ID: planID}}, workerAssignment: assignment}
	raw, err := json.Marshal(map[string]any{"id": "sub_new", "customer": "cus_new", "status": "active", "metadata": map[string]string{"org_id": orgID.String()}, "items": map[string]any{"data": []any{map[string]any{"price": map[string]string{"id": "price_new"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if xerr := s.handleSubscriptionUpdated(context.Background(), &stripeapi.Event{Data: &stripeapi.EventData{Raw: raw}}); xerr != nil {
		t.Fatal(xerr)
	}
	if !repo.updated || repo.sub.PlanID != planID || repo.sub.StripeCustomerID != "cus_new" || repo.sub.StripeSubscriptionID == nil || *repo.sub.StripeSubscriptionID != "sub_new" || repo.sub.Status != models.SubscriptionStatusActive {
		t.Fatalf("subscription event did not activate the workspace: %+v", repo.sub)
	}
	if assignment.org != orgID || assignment.pool != "premium" {
		t.Fatalf("mailboxes moved to %q for %s, want premium for %s", assignment.pool, assignment.org, orgID)
	}
}

func TestSubscriptionLookupFailureRemainsRetryable(t *testing.T) {
	s := &stripeService{subRepo: &billingSubRepo{lookupErr: errors.New("database unavailable")}}
	err := s.handleSubscriptionUpdated(context.Background(), &stripeapi.Event{Data: &stripeapi.EventData{Raw: json.RawMessage(`{"id":"sub_new"}`)}})
	if err == nil || err.Code != errx.Internal {
		t.Fatalf("expected retryable failure, got %v", err)
	}
}

func TestAnnualChangeDoesNotFallBackToMonthly(t *testing.T) {
	s := &stripeService{subRepo: &billingSubRepo{sub: &models.Subscription{StripeSubscriptionID: stripeapi.String("sub_existing")}}, planRepo: &billingPlanRepo{plan: &models.Plan{StripePriceID: stripeapi.String("price_monthly")}}}
	_, err := s.ChangePlan(context.Background(), uuid.New(), uuid.New(), "", "", "year")
	if err == nil || err.Code != errx.BadRequest {
		t.Fatalf("expected unavailable annual price, got %v", err)
	}
}

type billingCredits struct {
	resetErr error
	granted  int
	reset    int
}

type billingTopUpAttempts struct {
	attempt *models.CreditAutoTopUpAttempt
}

func (r *billingTopUpAttempts) GetOrCreatePending(_ context.Context, orgID uuid.UUID, packKey string, credits int) (*models.CreditAutoTopUpAttempt, error) {
	if r.attempt == nil || r.attempt.Status != "pending" {
		r.attempt = &models.CreditAutoTopUpAttempt{
			ID:      uuid.New(),
			OrgID:   orgID,
			PackKey: packKey,
			Credits: credits,
			Status:  "pending",
		}
	}
	return r.attempt, nil
}

func (r *billingTopUpAttempts) SetTaxCalculation(_ context.Context, _ uuid.UUID, id string) (*models.CreditAutoTopUpAttempt, error) {
	r.attempt.TaxCalculationID = id
	return r.attempt, nil
}

func (r *billingTopUpAttempts) SetPaymentIntent(_ context.Context, _ uuid.UUID, id string) (*models.CreditAutoTopUpAttempt, error) {
	r.attempt.PaymentIntentID = id
	return r.attempt, nil
}

func (r *billingTopUpAttempts) MarkSucceededByPaymentIntent(_ context.Context, id string) error {
	if r.attempt != nil && r.attempt.PaymentIntentID == id {
		r.attempt.Status = "succeeded"
	}
	return nil
}

func (r *billingTopUpAttempts) MarkFailed(_ context.Context, _ uuid.UUID, message string) error {
	r.attempt.Status = "failed"
	r.attempt.FailureMessage = message
	return nil
}

func (c *billingCredits) ResetMonthlyAllowance(_ context.Context, _ uuid.UUID, amount int, _ string) error {
	c.reset = amount
	return c.resetErr
}
func (c *billingCredits) GrantPurchased(_ context.Context, _ uuid.UUID, amount int, _, _ string) (int, error) {
	c.granted += amount
	return c.granted, nil
}
func (r *billingSubRepo) WebhookEventExists(context.Context, string) (bool, error) { return false, nil }
func (r *billingSubRepo) RecordWebhookEvent(context.Context, *models.StripeWebhookEvent) error {
	r.recorded = true
	return nil
}

func TestAutoTopUpCalculatesAndAssociatesTax(t *testing.T) {
	orgID := uuid.New()
	attempts := &billingTopUpAttempts{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prices/price_pack":
			_, _ = w.Write([]byte(`{"id":"price_pack","currency":"usd","unit_amount":1000,"tax_behavior":"exclusive","product":"prod_credits"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/customers/cus_test":
			_, _ = w.Write([]byte(`{"id":"cus_test","invoice_settings":{"default_payment_method":"pm_test"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/tax/calculations":
			if r.Header.Get("Idempotency-Key") != "warmbly-credit-auto-topup-tax-"+attempts.attempt.ID.String() {
				t.Errorf("unexpected tax idempotency key %q", r.Header.Get("Idempotency-Key"))
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("customer") != "cus_test" || r.Form.Get("line_items[0][amount]") != "1000" || r.Form.Get("line_items[0][product]") != "prod_credits" || r.Form.Get("line_items[0][tax_behavior]") != "exclusive" {
				t.Errorf("invalid tax calculation: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"id":"taxcalc_test","currency":"usd","amount_total":1200}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment_intents":
			if r.Header.Get("Idempotency-Key") != "warmbly-credit-auto-topup-payment-"+attempts.attempt.ID.String() {
				t.Errorf("unexpected payment idempotency key %q", r.Header.Get("Idempotency-Key"))
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("amount") != "1200" || r.Form.Get("hooks[inputs][tax][calculation]") != "taxcalc_test" || r.Form.Get("metadata[tax_calculation_id]") != "taxcalc_test" {
				t.Errorf("payment intent is not linked to its tax calculation: %v", r.Form)
			}
			_, _ = w.Write([]byte(`{"id":"pi_test","status":"succeeded","metadata":{"org_id":"` + orgID.String() + `","purpose":"credit_auto_topup","pack_key":"pack_500","credits":"500","tax_calculation_id":"taxcalc_test"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	old := stripeapi.GetBackend(stripeapi.APIBackend)
	stripeapi.SetBackend(stripeapi.APIBackend, stripeapi.GetBackendWithConfig(stripeapi.APIBackend, &stripeapi.BackendConfig{URL: stripeapi.String(server.URL), HTTPClient: server.Client()}))
	t.Cleanup(func() { stripeapi.SetBackend(stripeapi.APIBackend, old) })

	credits := &billingCredits{}
	s := &stripeService{
		cfg:           &config.StripeConfig{CreditPackPriceIDs: map[string]string{"pack_500": "price_pack"}},
		subRepo:       &billingSubRepo{sub: &models.Subscription{StripeCustomerID: "cus_test"}},
		credits:       credits,
		topupAttempts: attempts,
	}
	granted, err := s.AutoTopUpCredits(context.Background(), orgID, "pack_500", 500)
	if err != nil || !granted || credits.granted != 500 {
		t.Fatalf("granted=%v credits=%d err=%v", granted, credits.granted, err)
	}
	if attempts.attempt.Status != "succeeded" || attempts.attempt.PaymentIntentID != "pi_test" {
		t.Fatalf("attempt was not completed: %+v", attempts.attempt)
	}
}

func TestAutoTopUpResumesPersistedStripeObjects(t *testing.T) {
	orgID := uuid.New()
	attempts := &billingTopUpAttempts{attempt: &models.CreditAutoTopUpAttempt{
		ID:               uuid.New(),
		OrgID:            orgID,
		PackKey:          "pack_500",
		Credits:          500,
		Status:           "pending",
		TaxCalculationID: "taxcalc_saved",
		PaymentIntentID:  "pi_saved",
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/prices/price_pack":
			_, _ = w.Write([]byte(`{"id":"price_pack","currency":"usd","unit_amount":1000,"tax_behavior":"exclusive","product":"prod_credits"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/customers/cus_test":
			_, _ = w.Write([]byte(`{"id":"cus_test","invoice_settings":{"default_payment_method":"pm_test"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/tax/calculations/taxcalc_saved":
			_, _ = w.Write([]byte(`{"id":"taxcalc_saved","currency":"usd","amount_total":1200}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/payment_intents/pi_saved":
			_, _ = w.Write([]byte(`{"id":"pi_saved","status":"succeeded","metadata":{"org_id":"` + orgID.String() + `","purpose":"credit_auto_topup","pack_key":"pack_500","credits":"500","tax_calculation_id":"taxcalc_saved"}}`))
		case r.Method == http.MethodPost:
			t.Errorf("retry created a new Stripe object: %s", r.URL.Path)
			http.Error(w, "unexpected create", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	old := stripeapi.GetBackend(stripeapi.APIBackend)
	stripeapi.SetBackend(stripeapi.APIBackend, stripeapi.GetBackendWithConfig(stripeapi.APIBackend, &stripeapi.BackendConfig{URL: stripeapi.String(server.URL), HTTPClient: server.Client()}))
	t.Cleanup(func() { stripeapi.SetBackend(stripeapi.APIBackend, old) })

	credits := &billingCredits{}
	s := &stripeService{
		cfg:           &config.StripeConfig{CreditPackPriceIDs: map[string]string{"pack_500": "price_pack"}},
		subRepo:       &billingSubRepo{sub: &models.Subscription{StripeCustomerID: "cus_test"}},
		credits:       credits,
		topupAttempts: attempts,
	}
	granted, err := s.AutoTopUpCredits(context.Background(), orgID, "pack_500", 500)
	if err != nil || !granted || credits.granted != 500 {
		t.Fatalf("granted=%v credits=%d err=%v", granted, credits.granted, err)
	}
	if attempts.attempt.Status != "succeeded" {
		t.Fatalf("attempt was not completed: %+v", attempts.attempt)
	}
}

func TestInvoiceBeforeCheckoutAndRetryAfterCreditFailure(t *testing.T) {
	orgID := uuid.New()
	repo := &billingSubRepo{sub: &models.Subscription{OrganizationID: orgID}}
	credits := &billingCredits{resetErr: errors.New("temporary ledger failure")}
	s := &stripeService{subRepo: repo, planRepo: &billingPlanRepo{plan: &models.Plan{MonthlyCredits: 500}}, credits: credits}
	raw := json.RawMessage(`{"subscription":"sub_new","billing_reason":"subscription_create","subscription_details":{"metadata":{"org_id":"` + orgID.String() + `"}},"lines":{"data":[{"price":{"id":"price_new"}}]}}`)
	event := &stripeapi.Event{ID: "evt_invoice", Type: "invoice.paid", Data: &stripeapi.EventData{Raw: raw}}
	if err := s.ProcessWebhookEvent(context.Background(), event); err == nil || err.Code != errx.Internal {
		t.Fatalf("expected retryable error, got %v", err)
	}
	if repo.recorded {
		t.Fatal("failed credit grant marked processed")
	}
	credits.resetErr = nil
	if err := s.ProcessWebhookEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if !repo.recorded || credits.reset != 500 {
		t.Fatal("retried invoice did not grant monthly allowance")
	}
}

func TestAsyncCreditPurchaseStoresCustomer(t *testing.T) {
	orgID := uuid.New()
	repo := &billingSubRepo{sub: &models.Subscription{OrganizationID: orgID}}
	credits := &billingCredits{}
	s := &stripeService{subRepo: repo, credits: credits}
	raw := json.RawMessage(`{"id":"cs_credit","customer":"cus_credit","payment_status":"paid","metadata":{"purpose":"credit_topup","credits":"500","org_id":"` + orgID.String() + `"}}`)
	event := &stripeapi.Event{ID: "evt_credit", Type: "checkout.session.async_payment_succeeded", Data: &stripeapi.EventData{Raw: raw}}
	if err := s.ProcessWebhookEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if !repo.updated || repo.sub.StripeCustomerID != "cus_credit" || credits.granted != 500 || !repo.recorded {
		t.Fatal("purchase did not persist the billing customer and credits")
	}
}
