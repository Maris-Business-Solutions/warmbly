// Roles & access — workspace role management.
//
// Roles are data: every workspace starts with seeded Admin / Manager /
// Viewer rows that can be renamed, recolored, reshaped, or deleted like any
// other role. Owner is a membership status, not a role.

import { LockIcon } from "lucide-react";
import { Link } from "@tanstack/react-router";
import useFeatureAccess from "@/hooks/useFeatureAccess";
import { useAppStore } from "@/stores";
import { Section, SectionShell } from "../_components/SectionShell";
import RolesSection from "./RolesSection";

export default function RolesSettingsPage() {
    const access = useFeatureAccess();
    const currentOrg = useAppStore((s) => s.currentOrganization);

    if (!access.loading && !access.canManage) {
        return (
            <SectionShell title="Roles & access" description="Team managers only.">
                <Section eyebrow="Permission denied">
                    <div className="flex items-start gap-3">
                        <div className="size-9 rounded-md bg-amber-50 border border-amber-200 text-amber-700 flex items-center justify-center shrink-0">
                            <LockIcon className="w-4 h-4" />
                        </div>
                        <div>
                            <div className="text-[13px] font-semibold text-slate-900">
                                You need team management access to manage roles
                            </div>
                            <p className="text-[12px] text-slate-500 leading-relaxed mt-1 max-w-md">
                                Roles control who can do what inside this workspace. Ask someone
                                with team access to review or change your permissions.
                            </p>
                        </div>
                    </div>
                </Section>
            </SectionShell>
        );
    }

    return (
        <SectionShell
            title="Roles & access"
            description={`What each role can do inside ${currentOrg?.name ?? "this workspace"}.`}
        >
            <Section
                eyebrow="Workspace roles"
                description="Every workspace starts with Admin, Manager, and Viewer. Rename, recolor, reshape, or delete them — and add your own. Editing a role updates everyone assigned to it."
            >
                <RolesSection canManage={access.canManage} />
                <p className="text-[11.5px] text-slate-500 leading-relaxed">
                    Assign roles from the member roster or the invite flow.{" "}
                    <Link
                        to="/app/settings/members"
                        className="text-slate-700 underline-offset-2 hover:underline"
                    >
                        Open Members →
                    </Link>
                </p>
            </Section>
        </SectionShell>
    );
}
