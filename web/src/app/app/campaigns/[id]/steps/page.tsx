import React from "react";
import { useCampaign } from "@/hooks/context/campaign";
import CampaignFlow from "@/components/app/campaigns/sequences/CampaignFlow";

export default function CampaignSteps() {
    const campaign = useCampaign();
    if (!campaign) {
        throw new Error("CampaignSteps cannot be rendered without a campaign");
    }

    // Keyed by campaign: a param-only navigation (e.g. jump-to-teammate from
    // one campaign's steps to another's) must remount the canvas, never reuse
    // one seeded from the previous campaign.
    return (
        <React.Suspense fallback={<StepsSkeleton />}>
            <CampaignFlow key={campaign.id} campaignId={campaign.id} />
        </React.Suspense>
    );
}

function StepsSkeleton() {
    return <div className="h-[74dvh] w-full animate-pulse rounded-md border border-slate-200 bg-slate-100/60" />;
}
