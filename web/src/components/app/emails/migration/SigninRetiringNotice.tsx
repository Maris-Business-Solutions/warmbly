// The mailbox drawer's optional alternatives to per-mailbox Google
// sign-in, with the move that fits it: onto the domain's admin grant, the
// whole-domain setup first, or an app password right here.
import React from "react";
import toast from "react-hot-toast/headless";
import { ArrowRightIcon, Building2Icon, Loader2Icon } from "lucide-react";
import ProviderLogo from "@/components/app/emails/ProviderLogo";
import { MoveError, useGrantConfig, useMoveToGrant, useSigninMigration } from "@/lib/api/hooks/app/emails/useMailboxGrants";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";
import { cn } from "@/lib/utils";
import MailboxGrantDialog from "../import/grants/MailboxGrantDialog";
import MailboxImportDialog from "../import/MailboxImportDialog";
import AppPasswordSwitchForm from "./AppPasswordSwitchForm";
import { migrationRoute } from "./migrationRoute";

export default function SigninRetiringNotice({ mailbox }: { mailbox: { id: string; email: string } }) {
    const migration = useSigninMigration();
    const googleGrants = useGrantConfig().data?.google_enabled === true;
    const move = useMoveToGrant();
    const [setupDomain, setSetupDomain] = React.useState<string | null>(null);
    const [importId, setImportId] = React.useState<string | null>(null);

    const group = migration.data?.data.find((g) => g.mailboxes.some((m) => m.id === mailbox.id));
    // The dialogs stay mounted after the move, while the notice itself goes away.
    const dialogs = (
        <>
            <MailboxGrantDialog
                open={!!setupDomain}
                provider="google"
                initialDomain={setupDomain ?? undefined}
                onClose={() => setSetupDomain(null)}
            />
            <MailboxImportDialog importId={importId} onClose={() => setImportId(null)} />
        </>
    );
    if (!group) return dialogs;
    const route = migrationRoute(group, googleGrants);

    async function moveIt() {
        if (!group?.grant_id || move.isPending) return;
        try {
            const out = await move.mutateAsync({ grantId: group.grant_id, emails: [mailbox.email] });
            toast.success(`Moving ${mailbox.email} onto the admin grant`);
            setImportId(out.job.id);
        } catch (e) {
            toast.error(e instanceof MoveError ? e.message : buildError(e as AppError));
        }
    }

    return (
        <div className="px-5 py-4">
            <div className="rounded-md border border-amber-200 bg-amber-50/70 px-3 py-2.5 space-y-2">
                <div className="flex items-start gap-2.5">
                    <ProviderLogo id="google" size="sm" className="mt-px" />
                    <div className="min-w-0 flex-1">
                        <p className="text-[12.5px] font-medium text-amber-900">Other connection methods</p>
                        <p className="text-[11.5px] text-amber-800 leading-relaxed">
                            This mailbox uses Google sign-in. You can keep it, or switch to an admin grant or an app password.
                            Switching preserves its history, campaigns and warmup.
                        </p>
                    </div>
                </div>
                {route === "grant" && (
                    <button
                        type="button"
                        onClick={() => void moveIt()}
                        aria-disabled={move.isPending}
                        className={cn(
                            "h-7 px-2.5 rounded-md bg-slate-900 hover:bg-slate-800 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors",
                            move.isPending && "opacity-60",
                        )}
                    >
                        {move.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <ArrowRightIcon className="w-3 h-3" />}
                        Move to the admin grant for {group.domain}
                    </button>
                )}
                {route === "setup" && (
                    <div className="flex flex-wrap items-center gap-2">
                        <button
                            type="button"
                            onClick={() => setSetupDomain(group.domain)}
                            className="h-7 px-2.5 rounded-md bg-slate-900 hover:bg-slate-800 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors"
                        >
                            <Building2Icon className="w-3 h-3" />
                            Set up the whole domain
                        </button>
                        <span className="text-[11px] text-amber-800">Then pick this mailbox under Mailboxes to move it.</span>
                    </div>
                )}
                {route === "app_password" && <AppPasswordSwitchForm mailbox={mailbox} />}
            </div>
            {dialogs}
        </div>
    );
}
