package smtp

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

// silentSMTPS answers 587 everywhere and leaves 465 silent.
func silentSMTPS(ctx context.Context, addr string) (net.Conn, error) {
	if strings.HasSuffix(addr, ":465") {
		<-ctx.Done()
		return nil, fakeTimeout{}
	}
	a, _ := net.Pipe()
	return a, nil
}

// One server with a silent 465 is that server; two are this worker's network.
func TestEgress_TwoServersSilentOn465MeansBlockedHere(t *testing.T) {
	swapDial(t, silentSMTPS)
	for i, host := range []string{"smtp.one.example", "smtp.two.example"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		d, err := DialSubmission(ctx, nil, host, 465, models.MailSecurityTLS)
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		d.Conn.Close()
		if got, want := smtpsEgress.Blocked(), i == 1; got != want {
			t.Fatalf("after %d silent servers Blocked() = %v, want %v", i+1, got, want)
		}
	}
	if EgressCondition() == "" {
		t.Fatal("a blocked worker reports no fleet condition")
	}
}

// A refused 465 is the server answering, not evidence against the network.
func TestEgress_RefusedSMTPSIsNotEvidence(t *testing.T) {
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		if strings.HasSuffix(addr, ":465") {
			time.Sleep(fallbackHeadStart + 100*time.Millisecond)
			return nil, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
		}
		time.Sleep(fallbackHeadStart + 300*time.Millisecond)
		a, _ := net.Pipe()
		return a, nil
	})
	for _, host := range []string{"smtp.one.example", "smtp.two.example"} {
		d, err := DialSubmission(context.Background(), nil, host, 465, models.MailSecurityTLS)
		if err != nil {
			t.Fatalf("%s: %v", host, err)
		}
		d.Conn.Close()
	}
	if smtpsEgress.Blocked() {
		t.Fatal("refusals were counted as a blocked network")
	}
}

// Any 465 that connects clears what was learned.
func TestEgress_ReachableSMTPSClearsTheBlock(t *testing.T) {
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		if strings.HasSuffix(addr, ":587") {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}
		a, _ := net.Pipe()
		return a, nil
	})
	smtpsEgress.markSilent("smtp.one.example")
	smtpsEgress.markSilent("smtp.two.example")
	smtpsEgress.markOpen()
	if smtpsEgress.Blocked() {
		t.Fatal("markOpen left the block in place")
	}
	smtpsEgress.markSilent("smtp.one.example")
	smtpsEgress.markSilent("smtp.two.example")
	d, err := DialSubmission(context.Background(), nil, "smtp.three.example", 465, models.MailSecurityTLS)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	d.Conn.Close()
	if d.Port != PortSMTPS {
		t.Fatalf("got port %d, want the 465 that answered first", d.Port)
	}
	if smtpsEgress.Blocked() {
		t.Fatal("a 465 that connected did not clear the block")
	}
}

// Evidence ages out, so a worker does not blame its network forever.
func TestEgress_EvidenceExpires(t *testing.T) {
	now := time.Now()
	s := &egressState{now: func() time.Time { return now }}
	s.markSilent("smtp.one.example")
	s.markSilent("smtp.two.example")
	if !s.Blocked() {
		t.Fatal("two silent servers did not block")
	}
	now = now.Add(smtpsEvidenceTTL + time.Minute)
	if s.Blocked() {
		t.Fatal("stale evidence still blocks")
	}
}

// On a blocked worker a 465-only server fails as the block, at once on 587.
func TestEgress_BlockedWorkerNamesItsOwnNetwork(t *testing.T) {
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		if strings.HasSuffix(addr, ":465") {
			<-ctx.Done()
			return nil, fakeTimeout{}
		}
		return nil, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	})
	smtpsEgress.markSilent("smtp.one.example")
	smtpsEgress.markSilent("smtp.two.example")
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	_, err := DialSubmission(ctx, nil, "mail.only465.example", 465, models.MailSecurityTLS)
	if !errors.Is(err, ErrSMTPSEgressBlocked) {
		t.Fatalf("got %v, want ErrSMTPSEgressBlocked", err)
	}
	if !dialTimedOut(err) {
		t.Fatalf("got %v, want the timeout kept underneath", err)
	}
}

// A send through a blocked worker keeps the retryable code and says why.
func TestEgress_SendSaysTheBlockIsOnTheSendingSide(t *testing.T) {
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		if strings.HasSuffix(addr, ":465") {
			<-ctx.Done()
			return nil, fakeTimeout{}
		}
		return nil, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	})
	smtpsEgress.markSilent("smtp.one.example")
	smtpsEgress.markSilent("smtp.two.example")
	c := &Client{
		Email:       "sender@example.com",
		AuthType:    models.AuthPlain,
		Credentials: &models.Service{Host: "mail.only465.example", Port: 465, Security: models.MailSecurityTLS},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	mailErr := c.sendRaw(ctx, "sender@example.com", []string{"to@example.test"}, []byte("Subject: hi\r\n\r\nbody\r\n"))
	if mailErr == nil {
		t.Fatal("send succeeded through a blocked 465")
	}
	if string(mailErr.Code) != "SERVER_UNREACHABLE" {
		t.Fatalf("code = %q, want SERVER_UNREACHABLE", mailErr.Code)
	}
	for _, want := range []string{"465", "mail.only465.example", "sending side"} {
		if !strings.Contains(mailErr.Message, want) {
			t.Errorf("message %q does not mention %q", mailErr.Message, want)
		}
	}
}
