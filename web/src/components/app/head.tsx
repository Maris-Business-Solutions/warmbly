import { Link } from "@tanstack/react-router";
import { hrefTarget } from "@/lib/routerSearch";

export function Head({ children, icon }: { children: React.ReactNode, icon?: React.ReactNode }) {
    return <div className="flex items-center gap-2 text-slate-800 font-semibold mb-2">
        {icon && <span className="text-slate-500 shrink-0">{icon}</span>}
        <span className="flex items-center gap-2">{children}</span>
    </div>
}
export function HeadLink({ children, href }: { children: React.ReactNode, href: string }) {
    return <Link {...hrefTarget(href)}>
        {children}
    </Link>
}