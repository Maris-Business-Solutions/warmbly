import ProviderLogo from "@/components/app/emails/ProviderLogo";
import { mailHostLabel } from "@/lib/mailHost";
import { companyDomainOf } from "@/lib/companyLogo";
import CompanyLogo from "./CompanyLogo";

// ContactAvatar is the contact's company logo when they are on a company
// domain, their initials otherwise, with who hosts their inbox as a badge on
// its corner, so both read at a glance without a column of their own.
export default function ContactAvatar({ c }: { c: { first_name?: string; email: string; mail_host?: string } }) {
    const host = c.mail_host ? mailHostLabel(c.mail_host) : "";
    const domain = companyDomainOf(c.email, c.mail_host);
    const title = [domain, host && `${host} inbox`].filter(Boolean).join(" · ");
    const initials = (
        <span className="text-[10px] font-semibold text-slate-600 tracking-wide">
            {(c.first_name || c.email)?.slice(0, 2).toUpperCase()}
        </span>
    );
    return (
        <div className="relative shrink-0" title={title || undefined}>
            <div
                className={`w-7 h-7 rounded-full overflow-hidden flex items-center justify-center ring-1 ring-inset ${
                    domain ? "bg-white ring-slate-200" : "bg-gradient-to-br from-slate-100 to-slate-200/80 ring-slate-200/70"
                }`}
            >
                {domain ? <CompanyLogo domain={domain} className="size-[18px] object-contain" fallback={initials} /> : initials}
            </div>
            {c.mail_host && (
                <span className="absolute -bottom-0.5 -right-1 size-[15px] rounded-full bg-white shadow-[0_1px_2px_rgba(15,23,42,0.18)] ring-1 ring-slate-200/80 flex items-center justify-center">
                    <ProviderLogo id={c.mail_host} size="xs" framed={false} />
                </span>
            )}
            {title && <span className="sr-only">{title}</span>}
        </div>
    );
}
