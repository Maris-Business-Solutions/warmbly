// Why a warming mailbox is sending nothing: its last warmup email failed and
// no later one was confirmed delivered.

import { AlertTriangleIcon } from "lucide-react";
import type { WarmupSendFailure } from "@/lib/api/models/app/analytics/AccountStatus";

export default function WarmupSendFailureNote({ failure, cloud = false, className = "" }: { failure: WarmupSendFailure; cloud?: boolean; className?: string }) {
    return (
        <div className={`mt-2 flex items-start gap-1.5 text-[11.5px] leading-relaxed text-rose-700 ${className}`}>
            <AlertTriangleIcon className="w-3 h-3 mt-0.5 shrink-0" />
            <div className="min-w-0">
                <div>
                    {cloud ? "Warmbly Cloud could not send the last warmup email from this mailbox." : "The last warmup email from this mailbox was not sent."} None has been delivered since, so
                    today's count stays where it is.
                </div>
                <div className="mt-0.5 font-mono text-[11px] text-rose-600/90 break-words">{failure.message}</div>
                <div className="mt-0.5 text-slate-500">
                    {cloud
                        ? "Warmbly Cloud connects to your mail server from its own network, so a server that only accepts this instance's address, or a host only reachable from here, refuses it. Check the SMTP host, port and any IP allowlist."
                        : "Check the SMTP host, port and password in the mailbox's settings."}
                </div>
            </div>
        </div>
    );
}
