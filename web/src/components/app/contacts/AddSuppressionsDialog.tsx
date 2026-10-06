import React from "react";
import { AnimatePresence, motion } from "framer-motion";
import { BanIcon, GlobeIcon, MailIcon, TriangleAlertIcon, XIcon } from "lucide-react";
import toast from "react-hot-toast/headless";

import { Label, TextInput } from "@/components/ui/field";
import { Loading } from "@/components/loader";
import { useConfirm } from "@/hooks/context/confirm";
import { useAddSuppressions } from "@/lib/api/hooks/app/suppressions/useSuppressions";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

// Paste-to-add: one address or domain per line, comma or whitespace also
// accepted, so a column copied out of a spreadsheet works as is. Every value
// is classified as it is typed so the user sees what will land before adding.

type Parsed = { raw: string; value: string; kind: "email" | "domain" | "invalid" };

// Mirrors the server's classification: a value with an @ must be an address,
// anything else must be a bare host. The server is still the authority.
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const DOMAIN = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$/;
// The server takes at most this many entries per request.
const MAX_ENTRIES = 5000;
// Enough chips to eyeball a paste without rendering thousands of nodes.
const PREVIEW_CHIPS = 200;

function parseSuppressionInput(raw: string): Parsed[] {
    const seen = new Set<string>();
    const out: Parsed[] = [];
    for (const token of raw.split(/[\s,;]+/)) {
        const trimmed = token.trim();
        if (!trimmed) continue;
        const value = trimmed.toLowerCase().replace(/^@/, "");
        if (!value || seen.has(value)) continue;
        seen.add(value);
        const kind = value.includes("@") ? (EMAIL.test(value) ? "email" : "invalid") : DOMAIN.test(value) ? "domain" : "invalid";
        out.push({ raw: trimmed, value, kind });
    }
    return out;
}

export default function AddSuppressionsDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
    const add = useAddSuppressions();
    const confirm = useConfirm();
    const [raw, setRaw] = React.useState("");
    const [reason, setReason] = React.useState("");
    const textareaRef = React.useRef<HTMLTextAreaElement>(null);

    const parsed = React.useMemo(() => parseSuppressionInput(raw), [raw]);
    const emails = parsed.filter((p) => p.kind === "email");
    const domains = parsed.filter((p) => p.kind === "domain");
    const invalid = parsed.filter((p) => p.kind === "invalid");
    const valid = emails.length + domains.length;
    const overLimit = valid > MAX_ENTRIES;
    const busy = add.isPending;
    const dirty = raw.trim() !== "" || reason.trim() !== "";

    React.useEffect(() => {
        if (!open) return;
        setRaw("");
        setReason("");
        const t = setTimeout(() => textareaRef.current?.focus(), 60);
        return () => clearTimeout(t);
    }, [open]);

    const requestClose = React.useCallback(() => {
        if (busy) return;
        if (!dirty) {
            onClose();
            return;
        }
        confirm.show("Discard what you pasted? Nothing has been added yet.", async () => onClose());
    }, [busy, dirty, onClose, confirm]);

    React.useEffect(() => {
        if (!open) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.key !== "Escape") return;
            // The discard confirm or a floating picker owns this Escape.
            if (document.querySelector("[data-floating], [role='alertdialog']")) return;
            e.preventDefault();
            requestClose();
        };
        document.addEventListener("keydown", onKey);
        return () => document.removeEventListener("keydown", onKey);
    }, [open, requestClose]);

    async function submit() {
        if (valid === 0 || overLimit || busy) return;
        try {
            const res = await add.mutateAsync({
                entries: [...emails, ...domains].map((p) => ({ value: p.value })),
                reason: reason.trim() || undefined,
            });
            const skipped = res.skipped?.length ?? 0;
            toast.success(
                skipped
                    ? `Added ${res.added}, skipped ${skipped} that did not look like an address or domain`
                    : `Added ${res.added} to the suppression list`,
            );
            onClose();
        } catch (err) {
            toast.error(buildError(err as AppError));
        }
    }

    return (
        <AnimatePresence>
            {open && (
                <>
                    <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        transition={{ duration: 0.18 }}
                        className="fixed inset-0 z-50 bg-slate-900/40"
                        onMouseDown={requestClose}
                    />
                    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
                        <motion.form
                            role="dialog"
                            aria-modal="true"
                            aria-labelledby="add-suppressions-title"
                            initial={{ opacity: 0, scale: 0.97, y: 8 }}
                            animate={{ opacity: 1, scale: 1, y: 0 }}
                            exit={{ opacity: 0, scale: 0.97, y: 8 }}
                            transition={{ duration: 0.2 }}
                            onMouseDown={(e) => e.stopPropagation()}
                            onSubmit={(e) => {
                                e.preventDefault();
                                void submit();
                            }}
                            className="pointer-events-auto w-full max-w-[520px] max-h-[88dvh] flex flex-col rounded-lg border border-slate-200 bg-white shadow-[0_24px_60px_-12px_rgba(15,23,42,0.35)]"
                        >
                            <div className="px-5 h-14 flex items-center gap-3 border-b border-slate-200 shrink-0">
                                <div className="w-8 h-8 rounded-lg bg-rose-50 text-rose-600 flex items-center justify-center shrink-0">
                                    <BanIcon className="w-4 h-4" />
                                </div>
                                <div className="min-w-0 flex-1">
                                    <div id="add-suppressions-title" className="text-[13px] font-medium text-slate-900">
                                        Add to suppression list
                                    </div>
                                    <div className="text-[11px] text-slate-400">No campaign will email these again</div>
                                </div>
                                <button
                                    type="button"
                                    onClick={requestClose}
                                    disabled={busy}
                                    aria-label="Close"
                                    className="w-7 h-7 rounded-md flex items-center justify-center text-slate-400 hover:text-slate-900 hover:bg-slate-100 transition-colors disabled:opacity-50"
                                >
                                    <XIcon className="w-4 h-4" />
                                </button>
                            </div>

                            <div className="px-5 py-4 space-y-4 overflow-y-auto">
                                <div>
                                    <Label>Addresses or domains</Label>
                                    <textarea
                                        ref={textareaRef}
                                        value={raw}
                                        onChange={(e) => setRaw(e.target.value)}
                                        onKeyDown={(e) => {
                                            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                                                e.preventDefault();
                                                void submit();
                                            }
                                        }}
                                        rows={6}
                                        spellCheck={false}
                                        placeholder={"jane@acme.com\nacme.com\n@partner.io"}
                                        className="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-[12.5px] font-mono leading-5 text-slate-900 placeholder:text-slate-400 outline-none transition-colors focus:border-sky-400 focus:ring-2 focus:ring-sky-100 resize-y"
                                    />
                                    <p className="mt-1 text-[11px] text-slate-500">
                                        One per line, or paste a column. A bare domain suppresses every address at it.
                                    </p>
                                </div>

                                {parsed.length > 0 && (
                                    <div className="rounded-md border border-slate-200 bg-slate-50/60 divide-y divide-slate-200/70">
                                        <div className="px-3 h-8 flex items-center gap-3 text-[11px]">
                                            <Count icon={<MailIcon className="w-3 h-3" />} n={emails.length} word="address" plural="addresses" />
                                            <Count icon={<GlobeIcon className="w-3 h-3" />} n={domains.length} word="domain" plural="domains" />
                                            {invalid.length > 0 && (
                                                <span className="inline-flex items-center gap-1 text-rose-600 font-medium">
                                                    <TriangleAlertIcon className="w-3 h-3" />
                                                    {invalid.length} not recognized
                                                </span>
                                            )}
                                        </div>
                                        <div className="px-3 py-2 flex flex-wrap gap-1.5 max-h-28 overflow-y-auto">
                                            {parsed.slice(0, PREVIEW_CHIPS).map((p) => (
                                                <Chip key={p.value} parsed={p} />
                                            ))}
                                            {parsed.length > PREVIEW_CHIPS && (
                                                <span className="inline-flex items-center h-6 px-2 text-[11.5px] text-slate-500">
                                                    +{(parsed.length - PREVIEW_CHIPS).toLocaleString()} more
                                                </span>
                                            )}
                                        </div>
                                    </div>
                                )}

                                {overLimit && (
                                    <p className="text-[11px] text-rose-600">
                                        At most {MAX_ENTRIES.toLocaleString()} entries at a time. Split the list and add it in parts.
                                    </p>
                                )}

                                <div>
                                    <Label>Reason (optional)</Label>
                                    <TextInput value={reason} onChange={setReason} placeholder="Existing customer" maxLength={200} className="w-full" />
                                    <p className="mt-1 text-[11px] text-slate-500">Shown next to each entry so the team knows why it is here.</p>
                                </div>
                            </div>

                            <div className="px-5 h-14 flex items-center justify-between gap-2 border-t border-slate-200 bg-slate-50/60 shrink-0">
                                <span className="text-[11px] text-slate-400 hidden sm:inline">Ctrl/⌘ + Enter to add</span>
                                <div className="flex items-center gap-2 ml-auto">
                                    <button
                                        type="button"
                                        onClick={requestClose}
                                        disabled={busy}
                                        className="h-8 px-3 rounded-md border border-slate-200 hover:border-slate-300 text-[12px] text-slate-700 hover:text-slate-900 transition-colors disabled:opacity-50"
                                    >
                                        Cancel
                                    </button>
                                    <button
                                        type="submit"
                                        disabled={busy || valid === 0 || overLimit}
                                        className="h-8 px-3.5 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:opacity-60"
                                    >
                                        {busy && <Loading className="!w-3.5 h-3.5 text-white" />}
                                        {valid > 1 ? `Add ${valid}` : "Add"}
                                    </button>
                                </div>
                            </div>
                        </motion.form>
                    </div>
                </>
            )}
        </AnimatePresence>
    );
}

function Count({ icon, n, word, plural }: { icon: React.ReactNode; n: number; word: string; plural: string }) {
    return (
        <span className={`inline-flex items-center gap-1 ${n > 0 ? "text-slate-700" : "text-slate-400"}`}>
            {icon}
            {n} {n === 1 ? word : plural}
        </span>
    );
}

function Chip({ parsed }: { parsed: Parsed }) {
    const cls =
        parsed.kind === "invalid"
            ? "border-rose-200 bg-rose-50 text-rose-700 line-through decoration-rose-300"
            : parsed.kind === "domain"
              ? "border-sky-200 bg-sky-50 text-sky-700"
              : "border-slate-200 bg-white text-slate-700";
    const Icon = parsed.kind === "invalid" ? TriangleAlertIcon : parsed.kind === "domain" ? GlobeIcon : MailIcon;
    return (
        <span
            title={parsed.kind === "invalid" ? "Not an address or domain; it will be skipped" : undefined}
            className={`inline-flex items-center gap-1 h-6 max-w-full px-2 rounded-md border text-[11.5px] font-mono ${cls}`}
        >
            <Icon className="w-3 h-3 shrink-0" />
            <span className="truncate">{parsed.kind === "domain" ? `@${parsed.value}` : parsed.raw}</span>
        </span>
    );
}
