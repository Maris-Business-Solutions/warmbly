// Small pieces the import wizard's steps share.
import React from "react";
import { AlertTriangleIcon, CheckIcon, CopyIcon, EyeIcon, EyeOffIcon } from "lucide-react";
import toast from "react-hot-toast/headless";
import { TextInput } from "@/components/ui/field";
import ProviderLogo from "@/components/app/emails/ProviderLogo";
import { cn } from "@/lib/utils";

export function Banner({
    tone,
    title,
    children,
}: {
    tone: "amber" | "red" | "sky";
    title: string;
    children?: React.ReactNode;
}) {
    const cls = {
        red: "border-red-200 bg-red-50 text-red-900",
        amber: "border-amber-200 bg-amber-50 text-amber-900",
        sky: "border-sky-200 bg-sky-50 text-sky-900",
    }[tone];
    const body = {
        red: "text-red-800/90",
        amber: "text-amber-800/90",
        sky: "text-sky-800/90",
    }[tone];
    return (
        <div className={cn("rounded-md border px-3 py-2.5 flex items-start gap-2", cls)}>
            <AlertTriangleIcon className="w-3.5 h-3.5 mt-px shrink-0 opacity-80" />
            <div className="min-w-0">
                <p className="text-[12.5px] font-medium">{title}</p>
                {children && <div className={cn("text-[11.5px] leading-relaxed mt-0.5", body)}>{children}</div>}
            </div>
        </div>
    );
}

export type StatAccent = "emerald" | "slate" | "red" | "sky" | "amber";

export function StatCard({
    label,
    value,
    accent,
    active,
    onClick,
}: {
    label: string;
    value: number;
    accent: StatAccent;
    active?: boolean;
    onClick?: () => void;
}) {
    const ring = {
        emerald: "ring-emerald-200 bg-emerald-50 text-emerald-700",
        slate: "ring-slate-200 bg-slate-50 text-slate-700",
        red: "ring-red-200 bg-red-50 text-red-700",
        sky: "ring-sky-200 bg-sky-50 text-sky-700",
        amber: "ring-amber-200 bg-amber-50 text-amber-700",
    }[accent];
    const inner = (
        <>
            <div className="text-[10px] uppercase tracking-[0.14em] font-medium opacity-75 truncate">{label}</div>
            <div className="text-[18px] font-semibold tabular-nums mt-0.5">{value.toLocaleString()}</div>
        </>
    );
    if (!onClick) return <div className={cn("rounded-md ring-1 p-2.5 min-w-0", ring)}>{inner}</div>;
    return (
        <button
            type="button"
            onClick={onClick}
            aria-pressed={active}
            className={cn(
                "rounded-md ring-1 p-2.5 min-w-0 text-left transition-shadow",
                ring,
                active ? "ring-2 shadow-sm" : "hover:shadow-sm",
            )}
        >
            {inner}
        </button>
    );
}

export function SectionLabel({ children, className }: { children: React.ReactNode; className?: string }) {
    return (
        <div className={cn("text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium", className)}>{children}</div>
    );
}

export function Pill({ children, className, title }: { children: React.ReactNode; className?: string; title?: string }) {
    return (
        <span
            title={title}
            className={cn("inline-flex items-center h-[18px] px-1.5 rounded text-[10.5px] font-medium whitespace-nowrap", className)}
        >
            {children}
        </span>
    );
}

/** The host's logo, or a neutral glyph for an unknown host. */
export function HostMark({ host, className }: { host?: string | null; className?: string }) {
    return <ProviderLogo id={host} size="md" className={className} />;
}

const URL_RE = /(https?:\/\/[^\s,)]+[^\s,).])/g;

/** Text with its https links made clickable; fix text from the server names the pages to open. */
export function Linkified({ text }: { text: string }) {
    const parts = text.split(URL_RE);
    return (
        <>
            {parts.map((part, i) =>
                i % 2 === 1 ? (
                    <a
                        key={i}
                        href={part}
                        target="_blank"
                        rel="noreferrer"
                        className="text-sky-700 underline decoration-sky-300 hover:decoration-sky-600 break-all"
                    >
                        {part.replace(/^https?:\/\//, "")}
                    </a>
                ) : (
                    <React.Fragment key={i}>{part}</React.Fragment>
                ),
            )}
        </>
    );
}

/** A secret field with a show/hide toggle inside it. */
export function SecretInput({
    value,
    onChange,
    placeholder,
    autoFocus,
    invalid,
    inputClassName,
}: {
    value: string;
    onChange: (v: string) => void;
    placeholder?: string;
    autoFocus?: boolean;
    invalid?: boolean;
    inputClassName?: string;
}) {
    const [shown, setShown] = React.useState(false);
    return (
        <div className="relative min-w-0" data-ph-mask="">
            <TextInput
                value={value}
                onChange={onChange}
                type={shown ? "text" : "password"}
                autoComplete="off"
                placeholder={placeholder}
                autoFocus={autoFocus}
                invalid={invalid}
                className={cn("w-full pr-8", inputClassName)}
            />
            <button
                type="button"
                onClick={() => setShown((v) => !v)}
                aria-label={shown ? "Hide" : "Show"}
                title={shown ? "Hide" : "Show"}
                className="absolute right-1 top-1/2 -translate-y-1/2 size-5 rounded text-slate-400 hover:text-slate-700 hover:bg-slate-100 inline-flex items-center justify-center transition-colors"
            >
                {shown ? <EyeOffIcon className="w-3 h-3" /> : <EyeIcon className="w-3 h-3" />}
            </button>
        </div>
    );
}

/** A value to paste somewhere else, shown in full with a copy button. */
export function CopyValue({ label, value, display }: { label: string; value: string; display?: React.ReactNode }) {
    const [copied, setCopied] = React.useState(false);
    React.useEffect(() => {
        if (!copied) return;
        const t = window.setTimeout(() => setCopied(false), 1500);
        return () => window.clearTimeout(t);
    }, [copied]);
    const copy = async () => {
        try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
        } catch {
            toast.error("Could not copy. Select the text and copy it by hand.");
        }
    };
    return (
        <div className="rounded-md border border-slate-200 bg-slate-50/60 px-2.5 py-2 flex items-start gap-2 min-w-0">
            <div className="min-w-0 flex-1">
                <div className="text-[10px] uppercase tracking-[0.14em] text-slate-400 font-medium">{label}</div>
                <div className="mt-0.5 text-[11.5px] font-mono text-slate-800 break-all select-all">{display ?? value}</div>
            </div>
            <button
                type="button"
                onClick={() => void copy()}
                className={cn(
                    "shrink-0 h-6 px-2 rounded-md border text-[11.5px] font-medium inline-flex items-center gap-1 transition-colors",
                    copied ? "border-emerald-200 bg-emerald-50 text-emerald-700" : "border-slate-200 bg-white text-slate-700 hover:bg-slate-50",
                )}
            >
                {copied ? <CheckIcon className="w-3 h-3" /> : <CopyIcon className="w-3 h-3" />}
                {copied ? "Copied" : "Copy"}
            </button>
        </div>
    );
}

/** A row's status chip for a vendor account or an admin grant. */
export function SourceStatusPill({ status }: { status: "active" | "invalid" }) {
    return status === "active" ? (
        <Pill className="text-emerald-700 bg-emerald-50">Active</Pill>
    ) : (
        <Pill className="text-red-700 bg-red-50">Not working</Pill>
    );
}
