// Tester accounts — the operator view.
//
// A tester is an account handed to somebody outside the team: a vendor's
// reviewer during an OAuth verification, an auditor, a support engineer. It is
// an ordinary account marked exempt from the emailed login code, because the
// holder cannot read this instance's mail.
//
// It can either get a workspace of its own or join one that already exists. The
// second is for a review judged on the app doing real work, where an empty
// workspace shows none of it. Joining names a role explicitly: this is the one
// path that grants workspace access without anybody in that workspace asking
// for it, so there is no default.
//
// The list exists because the way this goes wrong is not creating one, it is
// forgetting it. An exemption taken out for a two-week review is still there a
// year later unless something shows it.

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { CopyIcon, FlaskConicalIcon, TrashIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminPerm } from "@/hooks/useAdminPerm";
import { AdminPerm } from "@/lib/auth/permissions";
import { createTester, listTesters, listOrganizationRoles, revokeTester } from "@/lib/api/client/admin/testers";
import { listOrganizations } from "@/lib/api/client/admin/organizations";
import { DASHBOARD_URL } from "@/lib/env";
import type { AdminOrgListItem, CreatedTester } from "@/lib/api/models/admin";

function fmt(ts?: string | null) {
    if (!ts) return "—";
    return new Date(ts).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

export default function TestersPage() {
    const qc = useQueryClient();
    const canManage = useAdminPerm(AdminPerm.ManageTesters);

    const [email, setEmail] = useState("");
    const [orgName, setOrgName] = useState("");
    const [reason, setReason] = useState("");
    const [mode, setMode] = useState<"new" | "existing">("new");
    const [orgQuery, setOrgQuery] = useState("");
    const [org, setOrg] = useState<AdminOrgListItem | null>(null);
    const [roleID, setRoleID] = useState("");
    const [passwordDays, setPasswordDays] = useState(30);
    // Held in state, never refetched: the server returns it once and cannot
    // produce it again.
    const [created, setCreated] = useState<CreatedTester | null>(null);

    const testers = useQuery({
        queryKey: ["admin", "testers"],
        queryFn: () => listTesters(),
    });

    // Only searched once there is something to search on: the unfiltered first
    // page is a list of arbitrary workspaces, which is not a picker.
    const orgs = useQuery({
        queryKey: ["admin", "testers", "orgs", orgQuery],
        queryFn: () => listOrganizations({ q: orgQuery.trim() }),
        enabled: mode === "existing" && orgQuery.trim().length >= 2,
    });

    const roles = useQuery({
        queryKey: ["admin", "testers", "roles", org?.id],
        queryFn: () => listOrganizationRoles(org!.id),
        enabled: mode === "existing" && !!org,
    });

    const joining = mode === "existing";
    const ready = !!email.trim() && !!reason.trim() && (!joining || (!!org && !!roleID));

    const create = useMutation({
        mutationFn: () =>
            createTester({
                email: email.trim(),
                reason: reason.trim(),
                password_days: passwordDays,
                ...(joining
                    ? { organization_id: org!.id, role_id: roleID }
                    : { org_name: orgName.trim() || undefined }),
            }),
        onSuccess: (t) => {
            setCreated(t);
            setEmail("");
            setOrgName("");
            setReason("");
            setOrgQuery("");
            setOrg(null);
            setRoleID("");
            setPasswordDays(30);
            qc.invalidateQueries({ queryKey: ["admin", "testers"] });
            toast.success("Tester created");
        },
        onError: (e: Error) => toast.error(e.message || "Could not create the tester"),
    });

    const revoke = useMutation({
        mutationFn: (id: string) => revokeTester(id),
        onSuccess: (r) => {
            qc.invalidateQueries({ queryKey: ["admin", "testers"] });
            toast.success(
                r.password_cleared
                    ? "Revoked: the password no longer works and every session is signed out"
                    : "Exemption revoked and every session signed out; the account now follows the instance policy",
            );
        },
        onError: (e: Error) => toast.error(e.message || "Could not revoke the exemption"),
    });

    function copy(text: string) {
        navigator.clipboard?.writeText(text).then(
            () => toast.success("Copied"),
            () => toast.error("Could not copy"),
        );
    }

    const rows = testers.data?.data ?? [];
    // A wrong sign-in address that looks plausible is the failure mode here: it
    // is copied straight into a vendor's verification form. The local default
    // surviving onto a deployed panel means nobody configured one.
    const looksUnset =
        /^https?:\/\/localhost(:|\/|$)/.test(DASHBOARD_URL) &&
        !/^https?:\/\/localhost(:|\/|$)/.test(window.location.origin);

    return (
        <div className="p-4 space-y-6">
            <div>
                <h1 className="text-sm font-semibold flex items-center gap-2">
                    <FlaskConicalIcon className="size-4 text-muted-foreground" />
                    Testers
                </h1>
                <p className="text-xs text-muted-foreground mt-1 max-w-[70ch]">
                    Accounts for people outside the team. Each skips the emailed login code, because the
                    holder cannot read this instance&apos;s mail. Everything else still applies: the password,
                    the captcha and the sign-in risk assessment. A tester either gets a workspace of its own
                    or joins one that already exists. Its password stops working on the date you choose, and
                    revoking it ends the password and signs out every session at once.
                </p>
            </div>

            {created && (
                <div className="border border-amber-200 bg-amber-50 rounded-lg p-3">
                    <div className="text-[12.5px] font-medium text-amber-900">
                        Copy these now. The password is not stored anywhere readable.
                    </div>
                    <div className="text-[11px] text-amber-800 mt-0.5">
                        {created.joined_existing
                            ? "This account is a member of an existing workspace and will land in it on sign-in."
                            : "This account owns a new, empty workspace."}
                    </div>
                    <dl className="mt-2 grid gap-1.5 text-xs">
                        {[
                            ["Sign in at", DASHBOARD_URL],
                            ["Email", created.email],
                            ["Password", created.password],
                            ["Expires", fmt(created.password_expires_at)],
                        ].map(([k, v]) => (
                            <div key={k} className="flex items-center gap-2">
                                <dt className="w-20 shrink-0 text-amber-800">{k}</dt>
                                <dd className="font-mono break-all" data-ph-mask={k === "Password" ? "" : undefined}>
                                    {v}
                                </dd>
                                <button
                                    type="button"
                                    onClick={() => copy(String(v))}
                                    className="shrink-0 rounded p-1 hover:bg-amber-100"
                                    aria-label={`Copy ${k}`}
                                >
                                    <CopyIcon className="size-3" />
                                </button>
                            </div>
                        ))}
                    </dl>
                    {looksUnset && (
                        <div className="text-[11px] text-red-700 mt-1.5">
                            That sign-in address is this panel&apos;s build-time default, not this
                            deployment&apos;s dashboard. Set <code>VITE_DASHBOARD_URL</code> (or{" "}
                            <code>WARMBLY_DASHBOARD_URL</code>) before sending it to anyone.
                        </div>
                    )}
                    <Button size="sm" variant="ghost" className="mt-2 h-7" onClick={() => setCreated(null)}>
                        Done
                    </Button>
                </div>
            )}

            {canManage && (
                <div className="border border-border rounded-lg bg-card p-3 space-y-2 max-w-[520px]">
                    <div className="text-[12.5px] font-medium">Create a tester</div>
                    <input
                        className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs"
                        placeholder="reviewer@example.com"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                    />

                    <div className="flex gap-1 pt-0.5">
                        {(["new", "existing"] as const).map((m) => (
                            <button
                                key={m}
                                type="button"
                                onClick={() => setMode(m)}
                                className={
                                    "h-7 px-2.5 rounded-md text-xs border " +
                                    (mode === m
                                        ? "border-foreground/20 bg-muted font-medium"
                                        : "border-border text-muted-foreground hover:bg-muted/50")
                                }
                            >
                                {m === "new" ? "New workspace" : "Join an existing workspace"}
                            </button>
                        ))}
                    </div>

                    {mode === "new" ? (
                        <input
                            className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs"
                            placeholder="Workspace name (optional)"
                            maxLength={64}
                            value={orgName}
                            onChange={(e) => setOrgName(e.target.value)}
                        />
                    ) : (
                        <div className="space-y-2">
                            <p className="text-[11px] text-muted-foreground leading-relaxed">
                                The tester becomes a member of this workspace and gets none of its own, so
                                signing in lands straight in it. It can see whatever the role below allows,
                                including real mailboxes and real contacts.
                            </p>

                            {org ? (
                                <div className="flex items-center gap-2 rounded-md border border-border px-2 h-8 text-xs">
                                    <span className="truncate font-medium">{org.name}</span>
                                    <span className="text-muted-foreground truncate">{org.owner_email}</span>
                                    <button
                                        type="button"
                                        className="ml-auto shrink-0 text-muted-foreground hover:text-foreground"
                                        onClick={() => {
                                            setOrg(null);
                                            setRoleID("");
                                        }}
                                    >
                                        Change
                                    </button>
                                </div>
                            ) : (
                                <>
                                    <input
                                        className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs"
                                        placeholder="Search workspaces by name or owner"
                                        value={orgQuery}
                                        onChange={(e) => setOrgQuery(e.target.value)}
                                    />
                                    {orgs.isError ? (
                                        <div className="text-[11px] text-red-600">
                                            Could not search workspaces. Retry before concluding there is no match.
                                        </div>
                                    ) : orgs.isFetching ? (
                                        <Skeleton className="h-8 w-full" />
                                    ) : (orgs.data?.data.length ?? 0) > 0 ? (
                                        <ul className="max-h-40 overflow-auto rounded-md border border-border divide-y divide-border">
                                            {orgs.data!.data.map((o) => (
                                                <li key={o.id}>
                                                    <button
                                                        type="button"
                                                        onClick={() => setOrg(o)}
                                                        className="w-full text-left px-2 py-1.5 text-xs hover:bg-muted/50"
                                                    >
                                                        <div className="font-medium truncate">{o.name}</div>
                                                        <div className="text-muted-foreground truncate">
                                                            {o.owner_email} · {o.member_count} members
                                                        </div>
                                                    </button>
                                                </li>
                                            ))}
                                        </ul>
                                    ) : orgQuery.trim().length >= 2 ? (
                                        <div className="text-[11px] text-muted-foreground">No workspace matches that.</div>
                                    ) : null}
                                </>
                            )}

                            {org && (
                                <select
                                    className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs"
                                    value={roleID}
                                    onChange={(e) => setRoleID(e.target.value)}
                                >
                                    <option value="">
                                        {roles.isFetching ? "Loading roles…" : "Choose a role…"}
                                    </option>
                                    {(roles.data?.data ?? []).map((r) => (
                                        <option key={r.id} value={r.id}>
                                            {r.name}
                                        </option>
                                    ))}
                                </select>
                            )}
                            {org && roles.isError && (
                                <div className="text-[11px] text-red-600">
                                    Could not load this workspace&apos;s roles, so there is nothing safe to pick.
                                </div>
                            )}
                        </div>
                    )}

                    <input
                        className="w-full h-8 rounded-md border border-border bg-background px-2 text-xs"
                        placeholder="Why this account exists, e.g. Google OAuth verification"
                        value={reason}
                        onChange={(e) => setReason(e.target.value)}
                    />
                    <label className="flex items-center gap-2 text-xs text-muted-foreground">
                        Password works for
                        <select
                            className="h-8 rounded-md border border-border bg-background px-2 text-xs text-foreground"
                            value={passwordDays}
                            onChange={(e) => setPasswordDays(Number(e.target.value))}
                        >
                            {[7, 14, 30, 60, 90].map((d) => (
                                <option key={d} value={d}>
                                    {d} days
                                </option>
                            ))}
                        </select>
                    </label>
                    <Button
                        size="sm"
                        className="h-8"
                        disabled={!ready || create.isPending}
                        onClick={() => create.mutate()}
                    >
                        Create
                    </Button>
                </div>
            )}

            <div className="border border-border rounded-lg bg-card">
                <div className="px-3 py-2 border-b border-border text-[12.5px] font-medium">
                    Active testers {rows.length > 0 && <Badge variant="outline" className="ml-1">{rows.length}</Badge>}
                </div>
                {testers.isLoading ? (
                    <div className="p-3"><Skeleton className="h-16 w-full" /></div>
                ) : testers.isError ? (
                    // Never fall through to the empty state here: "nothing is
                    // skipping the login code" is exactly the wrong thing to
                    // tell someone when the query failed.
                    <div className="p-3 text-xs text-red-600">
                        Could not load tester accounts, so this list is not authoritative. Retry before
                        concluding that none exist.
                    </div>
                ) : rows.length === 0 ? (
                    <div className="p-3 text-xs text-muted-foreground">
                        None. Nothing on this instance is skipping the login code.
                    </div>
                ) : (
                    <ul className="divide-y divide-border">
                        {rows.map((t) => (
                            <li key={t.user_id} className="flex items-center gap-3 px-3 py-2 text-xs">
                                <div className="flex-1 min-w-0">
                                    <div className="font-medium truncate">{t.email}</div>
                                    <div className="text-muted-foreground truncate">
                                        {t.reason || "no reason recorded"} · since {fmt(t.granted_at)}
                                        {t.password_expires_at &&
                                            ` · password ${
                                                new Date(t.password_expires_at) <= new Date() ? "expired" : "expires"
                                            } ${fmt(t.password_expires_at)}`}
                                    </div>
                                </div>
                                {canManage && (
                                    <Button
                                        size="sm"
                                        variant="outline"
                                        className="h-7 shrink-0"
                                        disabled={revoke.isPending}
                                        onClick={() => revoke.mutate(t.user_id)}
                                    >
                                        <TrashIcon className="size-3 mr-1" /> Revoke
                                    </Button>
                                )}
                            </li>
                        ))}
                    </ul>
                )}
            </div>
        </div>
    );
}
