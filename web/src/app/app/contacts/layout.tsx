// Contacts area: one tab strip over the contact list, saved segments and
// labels, since all three are views of the same contact database.

import { Link, Outlet, useLocation } from "@tanstack/react-router";
import { motion } from "framer-motion";
import { BanIcon, LayersIcon, TagIcon, UsersIcon } from "lucide-react";

import { NoAccess } from "@/components/layout/NoAccess";
import { usePermission } from "@/hooks/usePermission";

const TABS = [
    { label: "All contacts", path: "", Icon: UsersIcon },
    { label: "Segments", path: "/segments", Icon: LayersIcon },
    { label: "Labels", path: "/labels", Icon: TagIcon },
    { label: "Suppression list", path: "/suppressions", Icon: BanIcon },
] as const;

export default function ContactsLayout() {
    const canView = usePermission("VIEW_CONTACTS");
    const pathname = useLocation({ select: (l) => l.pathname });
    if (!canView) return <NoAccess feature="contacts" permissionLabel="View contacts" />;

    const current = pathname.replace(/\/$/, "");
    return (
        <div className="flex flex-col min-h-full">
            <div className="shrink-0 px-3 flex items-center gap-1 border-b border-slate-200 bg-white overflow-x-auto no-scrollbar">
                {TABS.map(({ label, path, Icon }) => {
                    const to = `/app/contacts${path}`;
                    const active = path === "" ? current === to : current.startsWith(to);
                    return (
                        <Link
                            key={path || "all"}
                            to={to}
                            className={`relative h-10 px-2.5 inline-flex items-center gap-1.5 text-[12.5px] transition-colors ${
                                active ? "text-slate-900 font-medium" : "text-slate-500 hover:text-slate-800"
                            }`}
                        >
                            <Icon className="w-3.5 h-3.5" />
                            {label}
                            {active && (
                                <motion.span
                                    layoutId="contacts-tab-underline"
                                    className="absolute left-1.5 right-1.5 -bottom-px h-0.5 rounded-full bg-sky-600"
                                    transition={{ type: "spring", duration: 0.3, bounce: 0.15 }}
                                />
                            )}
                        </Link>
                    );
                })}
            </div>
            <div className="flex-1 min-h-0 flex flex-col">
                <Outlet />
            </div>
        </div>
    );
}
