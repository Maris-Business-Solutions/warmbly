// useUpgradeFlow — the one place that turns "upgrade to Grow" into a Stripe
// action. The billing page and the upgrade dialog both go through it so the
// two surfaces cannot drift.
//
//   no active subscription  → Stripe Checkout (redirect), returns to `returnTo`
//   already on a paid plan  → in-place plan change with the chosen interval
//   enterprise              → contact sales
//   missing price           → explain that the plan is unavailable

import React from "react";
import toast from "react-hot-toast";
import useSubscription from "@/lib/api/hooks/app/subscription/useSubscription";
import useCreateCheckoutSession from "@/lib/api/hooks/app/subscription/useCreateCheckoutSession";
import useChangePlan from "@/lib/api/hooks/app/subscription/useChangePlan";
import useCreatePortalSession from "@/lib/api/hooks/app/subscription/useCreatePortalSession";
import usePlans from "@/lib/api/hooks/app/subscription/usePlans";
import type { AppError } from "@/lib/api/client/normalizeError";
import type ServerPlan from "@/lib/api/models/app/subscription/Plan";
import buildError from "@/lib/helper/buildError";
import { checkPermission, showPermissionDenied } from "@/hooks/usePermission";
import { getPlan, type PlanID } from "@/lib/plans";
import type { BillingInterval } from "@/lib/pricing";

export interface UpgradeOptions {
    interval: BillingInterval;
    /** Validated promo code to carry into checkout / the plan change. */
    discountCode?: string;
    /** In-app path Stripe returns to after checkout. Defaults to the billing plans tab. */
    returnTo?: string;
}

export type UpgradeOutcome = "redirect" | "changed" | "portal" | "contact" | "failed";

const DEFAULT_RETURN = "/app/settings/billing/plans";

function returnUrl(path: string, result: "success" | "cancel"): string {
    const u = new URL(path, window.location.origin);
    u.searchParams.set("checkout", result);
    return u.toString();
}

export default function useUpgradeFlow() {
    const subscription = useSubscription();
    const hasBillingCustomer = !!subscription.data?.stripe_customer_id?.trim();
    const hasStripeSubscription = !!subscription.data?.stripe_subscription_id?.trim() &&
        !["canceled", "incomplete_expired"].includes(subscription.data?.status ?? "");
    const plansQuery = usePlans();
    const checkout = useCreateCheckoutSession();
    const changePlan = useChangePlan();
    const portal = useCreatePortalSession();
    // Plan whose action is in flight; stays set through a redirect so the
    // button keeps its spinner until the page unloads.
    const [pending, setPending] = React.useState<PlanID | null>(null);

    // Resolve a catalog plan ("grow") to the server Plan record so we can read
    // its Stripe price ID / UUID. Undefined when the server has no matching
    // public plan configured.
    //
    // An exact name match always wins. Plans are operator-configurable, so a
    // public "Starter Legacy" ordered ahead of "Starter" would otherwise be
    // picked by the prefix pass and charge the wrong price. The prefix pass is
    // only a fallback, and only when exactly one plan matches.
    const resolveServerPlan = React.useCallback(
        (catalogId: PlanID): ServerPlan | undefined => {
            const label = getPlan(catalogId).label.toLowerCase().trim();
            const plans = (plansQuery.data ?? []) as ServerPlan[];
            const name = (p: ServerPlan) => (p.name ?? "").toLowerCase().trim();

            const exact = plans.find((p) => name(p) === label);
            if (exact) return exact;

            const prefixed = plans.filter((p) => name(p).startsWith(label));
            return prefixed.length === 1 ? prefixed[0] : undefined;
        },
        [plansQuery.data],
    );

    const openPortal = React.useCallback(async (): Promise<boolean> => {
        if (!checkPermission("MANAGE_BILLING")) {
            showPermissionDenied("MANAGE_BILLING");
            return false;
        }
        if (!hasBillingCustomer) {
            toast.error(subscription.isPending ? "Still loading billing. Try again in a moment." : "Complete checkout to set up billing before opening the portal.");
            return false;
        }
        try {
            // Stripe returns the browser to wherever the portal was opened from.
            const { url } = await toast.promise(portal.mutateAsync({ return_url: window.location.href }), {
                loading: "Opening billing portal…",
                success: "Portal ready",
                error: (e: AppError) => buildError(e),
            });
            window.location.assign(url);
            return true;
        } catch {
            return false;
        }
    }, [portal, hasBillingCustomer, subscription.isPending]);

    const upgrade = React.useCallback(
        async (catalogId: PlanID, opts: UpgradeOptions): Promise<UpgradeOutcome> => {
            if (pending) return "failed";
            // Checkout, plan changes and the portal all need Manage billing; say so before any request.
            if (!checkPermission("MANAGE_BILLING")) {
                showPermissionDenied("MANAGE_BILLING");
                return "failed";
            }
            // Wait for billing state before choosing checkout or a plan change.
            if (plansQuery.isPending || subscription.isPending) {
                toast.error("Still loading plans. Try again in a moment.");
                return "failed";
            }
            if (subscription.isError || plansQuery.isError) {
                toast.error("Could not load billing. Reload the page before trying again.");
                return "failed";
            }
            setPending(catalogId);
            let outcome: UpgradeOutcome = "failed";
            try {
                if (catalogId === "enterprise") {
                    outcome = "contact";
                    return outcome;
                }
                const target = resolveServerPlan(catalogId);
                const onPaid = hasStripeSubscription;
                const annual = opts.interval === "annual";
                const priceId = annual ? target?.stripe_price_id_yearly : target?.stripe_price_id;

                if (!target || !priceId) {
                    toast.error("This plan is not available for the selected billing interval. Please contact support.");
                    outcome = "failed";
                    return outcome;
                }

                if (onPaid) {
                    await toast.promise(
                        changePlan.mutateAsync({
                            plan_id: target.id,
                            discount_code: opts.discountCode,
                            interval: annual ? "year" : "month",
                        }),
                        {
                            loading: "Updating your plan…",
                            success: `You're on ${getPlan(catalogId).label} now`,
                            error: (e: AppError) => buildError(e),
                        },
                    );
                    outcome = "changed";
                    return outcome;
                }

                const back = opts.returnTo ?? DEFAULT_RETURN;
                const { checkout_url } = await toast.promise(
                    checkout.mutateAsync({
                        price_id: priceId as string,
                        success_url: returnUrl(back, "success"),
                        cancel_url: returnUrl(back, "cancel"),
                        discount_code: opts.discountCode,
                    }),
                    {
                        loading: "Starting checkout…",
                        success: "Redirecting to checkout…",
                        error: (e: AppError) => buildError(e),
                    },
                );
                window.location.assign(checkout_url);
                outcome = "redirect";
                return outcome;
            } catch {
                return "failed";
            } finally {
                // A redirect keeps its spinner until the page unloads.
                if (outcome !== "redirect") {
                    setPending((p) => (p === catalogId ? null : p));
                }
            }
        },
        [pending, plansQuery.isPending, plansQuery.isError, subscription.isPending, subscription.isError, hasStripeSubscription, resolveServerPlan, changePlan, checkout],
    );

    return {
        upgrade,
        openPortal,
        hasBillingCustomer,
        hasStripeSubscription,
        resolveServerPlan,
        /** Catalog plan whose upgrade is in flight, if any. */
        pending,
        portalPending: portal.isPending,
        plansLoading: plansQuery.isPending,
    };
}
