// The copy a placement test or batch sends, as a draft and as request fields,
// shared by both new-test dialogs.
import React from "react";
import { useQuery } from "@tanstack/react-query";
import { htmlToPlain } from "@/components/app/campaigns/sequences/emailPreview";
import getSequences from "@/lib/api/client/app/campaigns/sequences/getSequences";
import type Contact from "@/lib/api/models/app/contacts/Contact";

export type CopySource = "step" | "custom";

// Mirrors config.PlacementQuickSpacingSeconds.
export const QUICK_SPACING_SECONDS = 8;

export function newIdempotencyKey(): string {
    return typeof crypto !== "undefined" && "randomUUID" in crypto
        ? crypto.randomUUID()
        : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

// The copy a test sends: a saved campaign step or a template written here.
export interface CopyDraft {
    source: CopySource;
    campaignId: string;
    stepId: string;
    subject: string;
    bodyHtml: string;
    bodyPlain: string;
    bodyCode: boolean;
    contact: Contact | null;
}

// Whether the copy is complete enough to send, and what is missing when not.
export function copyIssue(d: CopyDraft, steps: { emailSteps: { id: string }[]; isLoading: boolean }): string | null {
    if (d.source === "step") {
        if (!d.campaignId) return "Pick a campaign.";
        if (!d.stepId) return steps.emailSteps.length === 0 && !steps.isLoading ? "This campaign has no email step to test." : "Pick a step.";
        return null;
    }
    if (!d.subject.trim()) return "Write a subject.";
    if (!(d.bodyPlain.trim() || (d.bodyCode && d.bodyHtml.trim()))) return "Write the email body.";
    return null;
}

// The copy fields of a request body.
export function copyBody(d: CopyDraft): {
    campaign_id?: string;
    sequence_id?: string;
    contact_id?: string;
    subject?: string;
    body_html?: string;
    body_plain?: string;
} {
    const contact = d.contact ? { contact_id: d.contact.id } : {};
    if (d.source === "step") return { campaign_id: d.campaignId, sequence_id: d.stepId, ...contact };
    return {
        subject: d.subject.trim(),
        body_html: d.bodyHtml,
        body_plain: d.bodyCode ? htmlToPlain(d.bodyHtml) : d.bodyPlain,
        ...contact,
    };
}

export function useCampaignEmailSteps(campaignId: string, enabled: boolean) {
    const steps = useQuery({
        queryKey: ["campaigns", campaignId, "sequences"],
        queryFn: () => getSequences(campaignId),
        enabled: enabled && !!campaignId,
    });
    const emailSteps = React.useMemo(
        () => (steps.data ?? []).filter((s) => (s.kind ?? "email") === "email"),
        [steps.data],
    );
    return { emailSteps, isLoading: steps.isLoading };
}
