// Community directory controls for one OAuth app: publish it, see where review
// stands, copy the share link, edit or unpublish. A published app is reachable by
// its link right away and joins the directory once an operator verifies it; any
// edit to what the listing shows sends it back for review.

import React from "react";
import { createPortal } from "react-dom";
import toast from "react-hot-toast";
import { AnimatePresence, motion } from "framer-motion";
import { BadgeCheckIcon, BlocksIcon, ClockIcon, CopyIcon, ExternalLinkIcon, Loader2Icon, ShieldXIcon, XIcon } from "lucide-react";

import { Label, TextInput } from "@/components/ui/field";
import { useConfirm } from "@/hooks/context/confirm";
import { useAppListing, useDeleteAppListing, useSaveAppListing } from "@/lib/api/hooks/app/oauth/useAppListing";
import {
    communityAppPath,
    LISTING_CATEGORY_LABELS,
    type AppListing,
    type AppListingCategory,
    type AppListingInput,
} from "@/lib/api/models/app/integrations/Community";
import type { OAuthApplication } from "@/lib/api/models/app/oauth/OAuthApp";
import { cn } from "@/lib/utils";

const CATEGORIES = Object.keys(LISTING_CATEGORY_LABELS) as AppListingCategory[];
const SLUG_RE = /^[a-z0-9](?:[a-z0-9]|-(?!-)){1,46}[a-z0-9]$/;

function slugify(name: string): string {
    return name
        .toLowerCase()
        .normalize("NFKD")
        .replace(/[^a-z0-9]+/g, "-")
        .replace(/^-+|-+$/g, "")
        .slice(0, 48)
        .replace(/-+$/g, "");
}

function shareLink(slug: string): string {
    return `${window.location.origin}${communityAppPath(slug)}`;
}

export default function AppListingPanel({ app }: { app: OAuthApplication }) {
    const listingQuery = useAppListing(app.id);
    const unpublish = useDeleteAppListing(app.id);
    const confirm = useConfirm();
    const [editing, setEditing] = React.useState(false);
    const listing = listingQuery.data?.listing ?? null;

    if (listingQuery.isPending) {
        return <div className="mt-2 h-10 rounded-md bg-slate-50 animate-pulse" />;
    }

    return (
        <div className="mt-2 rounded-md border border-slate-200 bg-slate-50/60 px-3 py-2">
            <div className="flex items-center gap-2 flex-wrap">
                <span className="text-[10px] uppercase tracking-[0.12em] text-slate-400">Community directory</span>
                {listing && <VerificationBadge listing={listing} />}
                <div className="ml-auto flex items-center gap-1">
                    {listing ? (
                        <>
                            <button
                                type="button"
                                onClick={() => {
                                    void navigator.clipboard.writeText(shareLink(listing.slug));
                                    toast.success("Link copied");
                                }}
                                className="h-6 px-2 rounded text-[11.5px] text-slate-600 hover:bg-white hover:text-slate-900 inline-flex items-center gap-1"
                            >
                                <CopyIcon className="w-3 h-3" />
                                Copy link
                            </button>
                            <button
                                type="button"
                                onClick={() => setEditing(true)}
                                className="h-6 px-2 rounded text-[11.5px] text-slate-600 hover:bg-white hover:text-slate-900"
                            >
                                Edit listing
                            </button>
                            <button
                                type="button"
                                onClick={() =>
                                    confirm.show(
                                        `Unpublish ${app.name}? Its link stops working and it leaves the directory. Workspaces that installed it keep their access.`,
                                        async () => {
                                            await unpublish.mutateAsync();
                                            toast.success("Unpublished");
                                        },
                                    )
                                }
                                className="h-6 px-2 rounded text-[11.5px] text-slate-500 hover:bg-rose-50 hover:text-rose-600"
                            >
                                Unpublish
                            </button>
                        </>
                    ) : (
                        <button
                            type="button"
                            onClick={() => setEditing(true)}
                            disabled={app.status !== "active"}
                            title={app.status !== "active" ? "Enable the app before publishing it" : undefined}
                            className="h-6 px-2 rounded bg-sky-600 hover:bg-sky-700 text-white text-[11.5px] font-medium inline-flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            <BlocksIcon className="w-3 h-3" />
                            Publish
                        </button>
                    )}
                </div>
            </div>
            <p className="mt-1 text-[11px] text-slate-500 leading-relaxed">
                {!listing
                    ? "List this app so other workspaces can find and install it."
                    : listing.verification === "verified"
                      ? "Listed in every workspace's Integrations page."
                      : listing.verification === "rejected"
                        ? "Hidden. Edit the listing to address the note below and send it for review again."
                        : "Waiting for review. Until it is verified, only people with the link can open it, and they see an unverified warning."}
            </p>
            {listing?.verification === "rejected" && listing.review_note && (
                <p className="mt-1.5 rounded border border-rose-200 bg-rose-50 px-2 py-1.5 text-[11.5px] text-rose-800 whitespace-pre-line">
                    {listing.review_note}
                </p>
            )}
            <AnimatePresence>
                {editing && <ListingModal key="listing" app={app} listing={listing} onClose={() => setEditing(false)} />}
            </AnimatePresence>
        </div>
    );
}

function VerificationBadge({ listing }: { listing: AppListing }) {
    const tone =
        listing.verification === "verified"
            ? { cls: "bg-emerald-50 text-emerald-700", icon: BadgeCheckIcon, label: "Verified" }
            : listing.verification === "rejected"
              ? { cls: "bg-rose-50 text-rose-700", icon: ShieldXIcon, label: "Not approved" }
              : { cls: "bg-amber-50 text-amber-700", icon: ClockIcon, label: "Unverified" };
    return (
        <span className={cn("inline-flex items-center gap-1 h-5 px-1.5 rounded text-[9.5px] uppercase tracking-[0.08em] font-medium", tone.cls)}>
            <tone.icon className="w-3 h-3" />
            {tone.label}
        </span>
    );
}

function ListingModal({ app, listing, onClose }: { app: OAuthApplication; listing: AppListing | null; onClose: () => void }) {
    const save = useSaveAppListing(app.id);
    const confirm = useConfirm();

    const initial: AppListingInput = React.useMemo(
        () => ({
            slug: listing?.slug ?? slugify(app.name),
            tagline: listing?.tagline ?? app.description.slice(0, 120),
            description: listing?.description ?? "",
            category: listing?.category ?? "other",
            install_url: listing?.install_url ?? app.website_url,
            support_url: listing?.support_url ?? "",
            privacy_url: listing?.privacy_url ?? "",
        }),
        [listing, app],
    );
    const [form, setForm] = React.useState<AppListingInput>(initial);
    const set = <K extends keyof AppListingInput>(k: K, v: AppListingInput[K]) => setForm((f) => ({ ...f, [k]: v }));
    const dirty = JSON.stringify(form) !== JSON.stringify(initial);

    const requestClose = React.useCallback(() => {
        if (dirty) confirm.show("Discard your changes to this listing?", async () => onClose());
        else onClose();
    }, [dirty, confirm, onClose]);

    React.useEffect(() => {
        function onKey(e: KeyboardEvent) {
            if (e.key !== "Escape") return;
            if (document.querySelector('[data-floating], [role="alertdialog"]')) return;
            requestClose();
        }
        window.addEventListener("keydown", onKey);
        return () => window.removeEventListener("keydown", onKey);
    }, [requestClose]);

    const slugValid = SLUG_RE.test(form.slug);
    const httpsOk = (v: string, required: boolean) => (v.trim() === "" ? !required : /^https:\/\/[^/\s]+/i.test(v.trim()));
    const problem = !slugValid
        ? "The link needs 3 to 48 lowercase letters, numbers or single dashes."
        : form.tagline.trim() === ""
          ? "Add a tagline."
          : !httpsOk(form.install_url, true)
            ? "The install URL must start with https://."
            : !httpsOk(form.support_url, false) || !httpsOk(form.privacy_url, false)
              ? "Support and privacy links must start with https://."
              : null;
    const resubmits = listing?.verification === "verified" && dirty;

    async function submit() {
        if (problem) {
            toast.error(problem);
            return;
        }
        try {
            await save.mutateAsync({
                ...form,
                slug: form.slug.trim(),
                tagline: form.tagline.trim(),
                description: form.description.trim(),
                install_url: form.install_url.trim(),
                support_url: form.support_url.trim(),
                privacy_url: form.privacy_url.trim(),
            });
            toast.success(listing ? "Listing saved" : "Published. Share the link while it waits for review.");
            onClose();
        } catch (e) {
            toast.error((e as { message?: string })?.message ?? "Could not save the listing");
        }
    }

    return createPortal(
        <div className="fixed inset-0 z-[60] flex items-center justify-center p-4">
            <motion.div
                className="absolute inset-0 bg-slate-900/40"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.12, ease: "easeOut" }}
                onMouseDown={requestClose}
            />
            <motion.div
                role="dialog"
                aria-modal="true"
                aria-label={listing ? "Edit listing" : "Publish to the community directory"}
                onMouseDown={(e) => e.stopPropagation()}
                className="relative w-full max-w-lg max-h-[90vh] flex flex-col overflow-hidden rounded-xl bg-white shadow-xl ring-1 ring-slate-200"
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: 20 }}
                transition={{ duration: 0.15, ease: "easeOut" }}
            >
                <div className="flex items-center border-b border-slate-200 px-4 h-11 shrink-0">
                    <span className="text-[12.5px] font-medium text-slate-900">
                        {listing ? `Edit ${app.name}'s listing` : `Publish ${app.name}`}
                    </span>
                    <button
                        type="button"
                        onClick={requestClose}
                        aria-label="Close"
                        className="ml-auto h-7 w-7 inline-flex items-center justify-center rounded-md text-slate-400 hover:bg-slate-100"
                    >
                        <XIcon className="w-4 h-4" />
                    </button>
                </div>

                <div className="flex-1 overflow-y-auto p-4 space-y-3.5">
                    <div className="rounded-md border border-sky-200 bg-sky-50/60 px-3 py-2 text-[11.5px] text-slate-600 leading-relaxed">
                        Your app's name, logo, website and permissions come from the app itself. Once published, anyone with the
                        link can open it with an unverified warning. It appears in the directory after it is reviewed, and any
                        change to the listing or the app sends it back for review.
                    </div>

                    <div>
                        <Label>Link</Label>
                        <div
                            className={cn(
                                "h-7 rounded-md border bg-white flex items-center text-[12px] overflow-hidden focus-within:ring-2",
                                slugValid || form.slug === ""
                                    ? "border-slate-200 focus-within:border-sky-400 focus-within:ring-sky-100"
                                    : "border-rose-300 focus-within:ring-rose-100",
                            )}
                        >
                            <span className="pl-2 text-slate-400 font-mono text-[11px] whitespace-nowrap">/integrations/apps/</span>
                            <input
                                value={form.slug}
                                onChange={(e) => set("slug", e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ""))}
                                maxLength={48}
                                className="flex-1 min-w-0 h-full pr-2 font-mono text-[11.5px] text-slate-900 outline-none bg-transparent"
                                aria-label="Link"
                            />
                        </div>
                    </div>

                    <div>
                        <Label>Tagline</Label>
                        <TextInput
                            value={form.tagline}
                            onChange={(v) => set("tagline", v)}
                            maxLength={120}
                            placeholder="Push positive replies into Acme as deals"
                            className="w-full"
                        />
                        <Counter value={form.tagline} max={120} />
                    </div>

                    <div>
                        <Label>Description</Label>
                        <textarea
                            value={form.description}
                            onChange={(e) => set("description", e.target.value)}
                            maxLength={2000}
                            rows={5}
                            placeholder="What it does, who it's for, and what happens after someone installs it."
                            className="w-full rounded-md border border-slate-200 bg-white px-2 py-1.5 text-[12px] text-slate-900 placeholder:text-slate-400 outline-none resize-y focus:border-sky-400 focus:ring-2 focus:ring-sky-100"
                        />
                        <Counter value={form.description} max={2000} />
                    </div>

                    <div>
                        <Label>Category</Label>
                        <div className="flex flex-wrap gap-1.5">
                            {CATEGORIES.map((c) => (
                                <button
                                    key={c}
                                    type="button"
                                    onClick={() => set("category", c)}
                                    className={cn(
                                        "h-7 px-2.5 rounded-full border text-[12px] transition-colors",
                                        form.category === c
                                            ? "border-sky-300 bg-sky-50 text-sky-800 font-medium"
                                            : "border-slate-200 text-slate-600 hover:border-slate-300",
                                    )}
                                >
                                    {LISTING_CATEGORY_LABELS[c]}
                                </button>
                            ))}
                        </div>
                    </div>

                    <div>
                        <Label>Install URL</Label>
                        <TextInput
                            value={form.install_url}
                            onChange={(v) => set("install_url", v)}
                            placeholder="https://acme.com/warmbly/install"
                            className="w-full"
                            invalid={!httpsOk(form.install_url, false)}
                        />
                        <p className="mt-1 text-[11px] text-slate-400">
                            Where Install takes people. Start the OAuth flow from there, with your own state.
                        </p>
                    </div>

                    <div className="grid sm:grid-cols-2 gap-3">
                        <div>
                            <Label>Support (optional)</Label>
                            <TextInput
                                value={form.support_url}
                                onChange={(v) => set("support_url", v)}
                                placeholder="https://acme.com/support"
                                className="w-full"
                                invalid={!httpsOk(form.support_url, false)}
                            />
                        </div>
                        <div>
                            <Label>Privacy policy (optional)</Label>
                            <TextInput
                                value={form.privacy_url}
                                onChange={(v) => set("privacy_url", v)}
                                placeholder="https://acme.com/privacy"
                                className="w-full"
                                invalid={!httpsOk(form.privacy_url, false)}
                            />
                        </div>
                    </div>

                    {listing && (
                        <a
                            href={communityAppPath(listing.slug)}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-[11.5px] text-slate-500 hover:text-sky-700 inline-flex items-center gap-1"
                        >
                            <ExternalLinkIcon className="w-3 h-3" />
                            See it as other workspaces do
                        </a>
                    )}
                </div>

                <div className="border-t border-slate-200 px-4 py-3 flex items-center gap-2 shrink-0">
                    <span className="text-[11px] text-slate-400 min-w-0 truncate">
                        {problem ?? (resubmits ? "Saving sends it back for review." : "")}
                    </span>
                    <div className="ml-auto flex items-center gap-2">
                        <button
                            type="button"
                            onClick={requestClose}
                            className="h-7 px-3 rounded-md border border-slate-200 text-[12px] text-slate-700 hover:border-slate-300"
                        >
                            Cancel
                        </button>
                        <button
                            type="button"
                            onClick={() => void submit()}
                            disabled={save.isPending || (!!listing && !dirty)}
                            className="h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 disabled:opacity-50"
                        >
                            {save.isPending && <Loader2Icon className="w-3.5 h-3.5 animate-spin" />}
                            {listing ? "Save" : "Publish"}
                        </button>
                    </div>
                </div>
            </motion.div>
        </div>,
        document.body,
    );
}

function Counter({ value, max }: { value: string; max: number }) {
    const n = [...value].length;
    return (
        <div className={cn("mt-1 text-right font-mono text-[10px] tabular-nums", n > max * 0.9 ? "text-amber-600" : "text-slate-300")}>
            {n}/{max}
        </div>
    );
}
