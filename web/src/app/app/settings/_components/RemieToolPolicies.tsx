// The workspace's "always allow" choices for Remie: which actions run without
// asking, who allowed them and when, and a way to make each one ask again.
// Settings managers only; the list refreshes live through the audit spine.

import toast from "react-hot-toast";
import { Loader2Icon, ShieldCheckIcon } from "lucide-react";
import { useRevokeToolPolicy, useToolPolicies } from "@/lib/api/hooks/app/agent/useToolPolicies";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { useConfirm } from "@/hooks/context/confirm";
import { toolAction } from "@/components/app/agent/toolLabels";

export default function RemieToolPolicies({ canManage }: { canManage: boolean }) {
    const policies = useToolPolicies(canManage);
    const revoke = useRevokeToolPolicy();
    const confirm = useConfirm();

    if (!canManage) return null;
    const rows = policies.data?.data ?? [];

    function onRevoke(tool: string) {
        confirm.show(`Make Remie ask before "${toolAction(tool)}" again?`, async () => {
            try {
                await revoke.mutateAsync(tool);
                toast.success("Remie will ask again");
            } catch (e) {
                toast.error(buildError(e as AppError));
            }
        });
    }

    return (
        <div className="pt-1">
            <div className="text-[12.5px] font-medium text-slate-900">Always allowed actions</div>
            <p className="mb-2 text-[11.5px] leading-relaxed text-slate-500">
                Actions Remie runs without asking, for everyone in the workspace. Sending, starting things that send, and
                changing access always ask and never appear here.
            </p>
            {policies.isPending ? (
                <div className="h-10 rounded bg-slate-100 animate-pulse" />
            ) : rows.length === 0 ? (
                <p className="text-[12px] text-slate-500">Nothing is always allowed. Every change asks first.</p>
            ) : (
                <div className="rounded-md border border-slate-200 overflow-hidden divide-y divide-slate-100 bg-white">
                    {rows.map((p) => (
                        <div key={p.tool_name} className="flex items-center gap-3 px-3 py-2">
                            <ShieldCheckIcon className="size-3.5 shrink-0 text-slate-400" />
                            <div className="min-w-0 flex-1">
                                <div className="truncate text-[12.5px] font-medium text-slate-900">{toolAction(p.tool_name)}</div>
                                <div className="truncate text-[11.5px] text-slate-500">
                                    {p.created_by_name ? `Allowed by ${p.created_by_name}` : "Allowed"}
                                    {" · "}
                                    {new Date(p.created_at).toLocaleDateString()}
                                </div>
                            </div>
                            <button
                                type="button"
                                onClick={() => onRevoke(p.tool_name)}
                                disabled={revoke.isPending && revoke.variables === p.tool_name}
                                className="h-7 shrink-0 rounded-md border border-slate-200 px-2.5 text-[12px] text-slate-700 hover:border-slate-300 hover:text-slate-900 inline-flex items-center gap-1.5 transition-colors disabled:opacity-50"
                            >
                                {revoke.isPending && revoke.variables === p.tool_name && (
                                    <Loader2Icon className="size-3 animate-spin" />
                                )}
                                Revoke
                            </button>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
