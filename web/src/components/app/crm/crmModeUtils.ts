// Provider-mode helpers for the CRM screens: error wording, owner mapping,
// sync time and stage probability.

import React from "react";
import useCrmProvider from "@/hooks/useCrmProvider";
import useCrmOwners from "@/lib/api/hooks/app/crm/provider/useCrmOwners";
import type { CRMExternalRef } from "@/lib/api/models/app/crm/CRMProvider";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

// Codes whose server message is already written for the person reading it.
const CRM_FALLBACK: Record<string, string> = {
    crm_provider_rejected: "Your CRM refused the change.",
    crm_reauth_required: "Reconnect your CRM to keep saving changes.",
    crm_unavailable: "Your CRM did not answer. Try again in a moment.",
    crm_owner_unmapped: "That member is not a user in your CRM. Match them in Integrations.",
    crm_stage_unknown: "That stage is no longer in your CRM. Pick another one.",
    crm_managed_externally: "This is managed in your CRM.",
};

// The message to show for a failed CRM write: the server's own sentence for a
// provider refusal, the usual "status: message" for anything else.
export function crmErrorMessage(err: unknown): string {
    const e = err as AppError;
    if (e?.code && Object.prototype.hasOwnProperty.call(CRM_FALLBACK, e.code)) return e.message || CRM_FALLBACK[e.code];
    return buildError(e);
}

// Which members the connected CRM knows as owners. Until the owners load (or
// outside provider mode) every member counts as mapped, so nothing is disabled
// on a guess.
export function useCrmOwnerIndex() {
    const { isExternal } = useCrmProvider();
    const { data, isSuccess } = useCrmOwners(isExternal);
    const mapped = React.useMemo(() => {
        const s = new Set<string>();
        for (const o of data ?? []) if (o.user_id && !o.archived) s.add(o.user_id);
        return s;
    }, [data]);
    const isMapped = React.useCallback(
        (userId: string) => !isExternal || !isSuccess || mapped.has(userId),
        [isExternal, isSuccess, mapped],
    );
    return { isExternal, isMapped };
}

// The newest synced_at across a set of mirrored records.
export function latestSyncedAt(items: { external?: CRMExternalRef }[]): Date | undefined {
    let max = 0;
    for (const it of items) {
        const t = it.external?.synced_at ? new Date(it.external.synced_at).getTime() : 0;
        if (t > max) max = t;
    }
    return max ? new Date(max) : undefined;
}

// Stage probability as the provider shows it ("20%"), stored as 0 to 1.
export function formatProbability(p?: number): string | null {
    if (p == null || !Number.isFinite(p)) return null;
    const pct = p <= 1 ? p * 100 : p;
    return `${Math.round(pct)}%`;
}

// The owner name to show for a provider record whose owner is not a member.
export function externalOwnerName(external?: CRMExternalRef): string | undefined {
    return external?.owner_name || undefined;
}
