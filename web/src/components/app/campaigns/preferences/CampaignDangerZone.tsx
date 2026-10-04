import { useNavigate } from "@tanstack/react-router";
import { TrashIcon } from "lucide-react";
import PermissionButton from "@/components/ui/PermissionButton";
import type Campaign from "@/lib/api/models/app/campaigns/Campaign";
import { useCampaignActions } from "@/components/app/campaigns/useCampaignActions";

// Settings > Delete campaign. Same confirm and cleanup as the "⋯" menu; on
// success the settings page no longer exists, so it lands on the list.
export default function CampaignDangerZone({ campaign }: { campaign: Campaign }) {
    const navigate = useNavigate();
    const actions = useCampaignActions();
    const running = campaign.status === "active";

    return (
        <div className="rounded-md border border-red-200 bg-red-50/40 p-4 flex flex-col sm:flex-row sm:items-center gap-3">
            <div className="min-w-0 flex-1">
                <p className="text-[12.5px] font-medium text-slate-900">Delete this campaign</p>
                <p className="text-[11.5px] text-slate-500 mt-0.5 leading-relaxed">
                    {running
                        ? "It is still running. Deleting stops sending immediately and removes the campaign, its steps, lead progress and activity. Contacts and emails already sent stay."
                        : "Removes the campaign, its steps, lead progress and activity. Contacts and emails already sent stay."}{" "}
                    This can't be undone.
                </p>
            </div>
            <PermissionButton
                permission="MANAGE_CAMPAIGNS"
                type="button"
                onClick={() =>
                    actions.requestDelete(campaign, { afterDelete: () => navigate({ to: "/app/campaigns" }) })
                }
                disabled={actions.deleting}
                className="h-7 px-2.5 rounded-md border border-red-200 text-red-600 hover:bg-red-600 hover:border-red-600 hover:text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60 shrink-0"
            >
                <TrashIcon className="w-3.5 h-3.5" />
                Delete campaign
            </PermissionButton>
        </div>
    );
}
