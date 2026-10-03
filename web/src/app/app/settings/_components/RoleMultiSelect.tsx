// Multi-role picker: checkbox dropdown + colored chips for the selected
// roles. A member can hold several roles; effective access is the union.

import React from "react";
import { CheckIcon, ChevronDownIcon, Loader2Icon } from "lucide-react";
import type OrganizationRole from "@/lib/api/models/app/organizations/OrganizationRole";
import type { MemberRole } from "@/lib/api/models/app/organizations/OrganizationMember";
import {
    PopoverMenu,
    PopoverMenuContent,
    PopoverMenuTrigger,
} from "@/components/ui/popover-menu";
import { roleColor } from "./RoleSelect";
import { labelInk } from "@/lib/utils";

export function RoleChips({ roles }: { roles: MemberRole[] }) {
    if (roles.length === 0) {
        return <span className="text-[11px] text-slate-400">No role</span>;
    }
    return (
        <span className="inline-flex flex-wrap items-center gap-1">
            {roles.map((r) => (
                <span
                    key={r.id}
                    className="inline-flex items-center gap-1 h-5 px-1.5 rounded text-[10px] font-medium border"
                    style={{ backgroundColor: `${r.color || "#64748b"}14`, borderColor: `${r.color || "#64748b"}55`, color: labelInk(r.color || "#475569") }}
                >
                    <span className="size-1.5 rounded-full" style={{ backgroundColor: r.color || "#64748b" }} />
                    {r.name}
                </span>
            ))}
        </span>
    );
}

export default function RoleMultiSelect({
    roles,
    value,
    onChange,
    pending = false,
    align = "start",
}: {
    roles: OrganizationRole[];
    /** Selected role ids. */
    value: string[];
    /** Fires with the next full set; never empty (at least one role). */
    onChange: (roleIds: string[]) => void;
    pending?: boolean;
    align?: "start" | "end";
}) {
    const [open, setOpen] = React.useState(false);
    const selected = roles.filter((r) => value.includes(r.id));
    const summary =
        selected.length === 0
            ? "Select roles"
            : selected.length === 1
                ? selected[0].name
                : `${selected[0].name} +${selected.length - 1}`;

    const toggle = (id: string) => {
        const next = value.includes(id) ? value.filter((v) => v !== id) : [...value, id];
        if (next.length === 0) return; // keep at least one
        onChange(next);
    };

    return (
        <PopoverMenu open={open} onOpenChange={setOpen} align={align}>
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    disabled={pending || roles.length === 0}
                    className="h-6 px-1.5 rounded text-[10px] uppercase tracking-[0.08em] font-semibold inline-flex items-center gap-1 border border-slate-200 bg-white text-slate-700 hover:bg-slate-50 transition-colors disabled:opacity-60"
                >
                    {pending ? (
                        <Loader2Icon className="w-2.5 h-2.5 animate-spin" />
                    ) : selected.length > 0 ? (
                        <span className="size-1.5 rounded-full" style={{ backgroundColor: roleColor(selected[0]) }} />
                    ) : null}
                    {summary}
                    <ChevronDownIcon className="w-2.5 h-2.5 opacity-60" />
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={208} className="max-w-[calc(100vw-2rem)]">
                {roles.map((r) => {
                    const on = value.includes(r.id);
                    return (
                        <button
                            key={r.id}
                            type="button"
                            title={r.description || undefined}
                            onClick={() => toggle(r.id)}
                            className="w-full px-2.5 py-1.5 text-left hover:bg-slate-100 transition-colors flex items-center gap-2"
                        >
                            <span
                                className={`size-3.5 rounded-sm border inline-flex items-center justify-center shrink-0 ${
                                    on ? "border-transparent" : "border-slate-300 bg-white"
                                }`}
                                style={on ? { backgroundColor: roleColor(r) } : undefined}
                            >
                                {on && <CheckIcon className="w-2.5 h-2.5 text-white" />}
                            </span>
                            <span className="size-1.5 rounded-full" style={{ backgroundColor: roleColor(r) }} />
                            <span className="text-[12px] font-medium text-slate-900">{r.name}</span>
                        </button>
                    );
                })}
                {roles.length === 0 && (
                    <div className="px-2.5 py-2 text-[11.5px] text-slate-500">
                        No roles yet. Create one under Settings → Roles & access.
                    </div>
                )}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}
