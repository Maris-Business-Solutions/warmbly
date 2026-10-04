// Shown on a campaign parked at paused_undeliverable: verification refused
// every remaining lead. Two ways out, both one click: re-check the leads, or
// trust a list verified elsewhere and send anyway.

import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { Link } from "@tanstack/react-router";
import { Loader2Icon, RefreshCcwIcon, SendIcon, ShieldAlertIcon } from "lucide-react";
import toast from "react-hot-toast";

import { useConfirm } from "@/hooks/context/confirm";
import PermissionButton from "@/components/ui/PermissionButton";
import { useRequestContactVerification } from "@/lib/api/hooks/app/contacts/useContactVerification";
import { reverifyNotice } from "@/lib/api/client/app/contacts/verification";
import useStartCampaign from "@/lib/api/hooks/app/campaigns/useStartCampaign";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

export default function UndeliverableBanner({
    campaignId,
    status,
}: {
    campaignId: string;
    status: string;
}) {
    const confirm = useConfirm();
    const request = useRequestContactVerification();
    const start = useStartCampaign();
    const [action, setAction] = React.useState<"verify" | "send" | null>(null);

    const reverify = async () => {
        setAction("verify");
        try {
            const res = await request.mutateAsync({ campaign_id: campaignId, action: "verify" });
            const notice = reverifyNotice(res, "lead", "leads");
            if (notice.warn) toast(notice.text, { icon: "⚠️" });
            else toast.success(`${notice.text}. Sending resumes as soon as any pass.`);
        } catch (e) {
            toast.error(buildError(e as AppError));
        } finally {
            setAction(null);
        }
    };

    const sendAnyway = () => {
        confirm?.show(
            "Send to the refused leads anyway? They are marked deliverable and the campaign resumes. Use this only for a list you verified elsewhere: sending to dead addresses costs you bounces.",
            async () => {
                setAction("send");
                try {
                    await request.mutateAsync({ campaign_id: campaignId, action: "mark_deliverable" });
                    await start.mutateAsync({ id: campaignId, options: { acknowledge_list_risk: true } });
                    toast.success("Campaign resumed");
                } catch (e) {
                    toast.error(buildError(e as AppError));
                } finally {
                    setAction(null);
                }
            },
        );
    };

    return (
        <AnimatePresence initial={false}>
            {status === "paused_undeliverable" && (
                <motion.div
                    key="undeliverable"
                    initial={{ opacity: 0, y: -8, height: 0 }}
                    animate={{ opacity: 1, y: 0, height: "auto" }}
                    exit={{ opacity: 0, y: -8, height: 0 }}
                    transition={{ type: "spring", duration: 0.4, bounce: 0.2 }}
                    className="overflow-hidden"
                >
                    <div className="mx-5 mb-3 rounded-md border border-amber-200 bg-amber-50/70 px-3.5 py-3 flex flex-col md:flex-row md:items-center gap-3">
                        <motion.span
                            initial={{ rotate: -12, scale: 0.8 }}
                            animate={{ rotate: 0, scale: 1 }}
                            transition={{ type: "spring", duration: 0.5, bounce: 0.5 }}
                            className="shrink-0 w-7 h-7 rounded-md bg-amber-100 text-amber-700 inline-flex items-center justify-center"
                        >
                            <ShieldAlertIcon className="w-4 h-4" />
                        </motion.span>
                        <div className="min-w-0 flex-1">
                            <p className="text-[12.5px] font-medium text-amber-900">
                                Sending paused: address verification refused the remaining leads
                            </p>
                            <p className="text-[11.5px] text-amber-800/80 leading-snug mt-0.5">
                                Re-check them now, or send anyway if you verified this list with another service.
                                Verification settings live under{" "}
                                <Link to="/app/settings/sending" className="underline underline-offset-2 hover:text-amber-900">
                                    Settings
                                </Link>
                                .
                            </p>
                        </div>
                        <div className="shrink-0 flex items-center gap-1.5">
                            <PermissionButton
                                permission="MANAGE_CONTACTS"
                                type="button"
                                onClick={reverify}
                                disabled={action !== null}
                                className="h-7 px-2.5 rounded-md border border-amber-300 bg-white hover:bg-amber-100 text-amber-900 text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                            >
                                {action === "verify" ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <RefreshCcwIcon className="w-3.5 h-3.5" />}
                                Re-verify leads
                            </PermissionButton>
                            <PermissionButton
                                permission="SEND_CAMPAIGNS"
                                type="button"
                                onClick={sendAnyway}
                                disabled={action !== null}
                                className="h-7 px-2.5 rounded-md bg-amber-600 hover:bg-amber-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                            >
                                {action === "send" ? <Loader2Icon className="w-3.5 h-3.5 animate-spin" /> : <SendIcon className="w-3.5 h-3.5" />}
                                Send anyway
                            </PermissionButton>
                        </div>
                    </div>
                </motion.div>
            )}
        </AnimatePresence>
    );
}
