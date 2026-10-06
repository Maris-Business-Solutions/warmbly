// Shared delete / duplicate flows for a campaign, so the list row menu, the
// detail header and the settings danger zone behave identically: same
// confirmation copy, same toasts, same permission gate, same landing page
// after a duplicate.

import { useNavigate } from "@tanstack/react-router";
import toast from "react-hot-toast/headless";
import { useConfirm } from "@/hooks/context/confirm";
import { checkPermission, showPermissionDenied, type PermissionKey } from "@/hooks/usePermission";
import useDeleteCampaign from "@/lib/api/hooks/app/campaigns/useDeleteCampaign";
import useDuplicateCampaign from "@/lib/api/hooks/app/campaigns/useDuplicateCampaign";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

export interface CampaignLike {
    id: string;
    name: string;
    status?: string | null;
}

// Statuses from which a start is accepted by the backend.
const STARTABLE = new Set(["draft", "paused", "paused_no_accounts", "paused_guardrail", "paused_undeliverable", "completed"]);

export function canStartCampaign(status: string | null | undefined): boolean {
    return STARTABLE.has(status ?? "draft");
}

// The confirmation spells out what is removed and, for a running campaign,
// that sending stops as part of the delete rather than requiring a pause.
export function deleteCampaignPrompt(c: CampaignLike): string {
    const what =
        "The campaign, its steps, lead progress and activity are permanently removed. Contacts and emails already sent stay.";
    if (c.status === "active") {
        return `Delete "${c.name}"? It is still running: sending stops immediately. ${what} This can't be undone.`;
    }
    return `Delete "${c.name}"? ${what} This can't be undone.`;
}

// gate runs fn when the member holds the permission and pops the standard
// permission explanation otherwise, for controls that are not buttons.
export function gate(key: PermissionKey, fn: () => void): () => void {
    return () => {
        if (!checkPermission(key)) {
            showPermissionDenied(key);
            return;
        }
        fn();
    };
}

export function useCampaignActions() {
    const confirm = useConfirm();
    const navigate = useNavigate();
    const del = useDeleteCampaign();
    const duplicate = useDuplicateCampaign();

    // Confirm, then delete. A failure keeps the dialog open (the provider
    // stays up when the callback throws) with the error in a toast, so the
    // user can retry or cancel; success closes it and runs afterDelete.
    function requestDelete(c: CampaignLike, opts?: { afterDelete?: () => void }) {
        confirm.show(deleteCampaignPrompt(c), async () => {
            await toast.promise(del.mutateAsync(c.id), {
                loading: "Deleting campaign…",
                success: `Deleted "${c.name}"`,
                error: (e: AppError) => buildError(e),
            });
            opts?.afterDelete?.();
        });
    }

    // Duplicate into a draft and land on its settings so it can be renamed
    // and adjusted before it is started.
    async function duplicateAndOpen(c: CampaignLike) {
        try {
            const created = await toast.promise(duplicate.mutateAsync({ id: c.id }), {
                loading: "Duplicating campaign…",
                success: (copy) => `Created "${copy.name}" as a draft`,
                error: (e: AppError) => buildError(e),
            });
            navigate({ to: "/app/campaigns/$id/preferences", params: { id: created.id } });
        } catch {
            /* toast.promise already surfaced */
        }
    }

    return {
        requestDelete,
        duplicateAndOpen,
        deleting: del.isPending,
        duplicating: duplicate.isPending,
    };
}
