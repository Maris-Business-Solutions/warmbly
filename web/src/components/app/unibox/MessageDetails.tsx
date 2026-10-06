// The envelope of one message, opened in place from its header. Every value
// is a button that copies it, which is what people open this for.

import React from "react";
import { motion } from "framer-motion";
import toast from "react-hot-toast/headless";
import { AlertCircleIcon, CheckIcon, CopyIcon } from "lucide-react";
import type UniboxEmail from "@/lib/api/models/app/unibox/UniboxEmail";
import type { UniboxEmailDetail } from "@/lib/api/models/app/unibox/UniboxEmail";
import { useAppStore } from "@/stores";
import formatBytes from "@/lib/helper/formatBytes";
import {
    addressParts,
    folderLabel,
    formatExactTime,
    receivedDiffers,
    recipientsOf,
    relativeTime,
    sameAddress,
} from "@/lib/unibox/messageDetails";
import { cn } from "@/lib/utils";

interface MessageDetailsProps {
    email: UniboxEmail;
    detail?: UniboxEmailDetail;
    /** The full fetch is still running: the rows that need it show a skeleton. */
    loading?: boolean;
    /** The full fetch failed: the rows that need it are named as missing. */
    error?: boolean;
}

export default function MessageDetails({ email, detail, loading = false, error = false }: MessageDetailsProps) {
    const accounts = useAppStore((s) => s.emails);
    const mailboxId = detail?.email_id || email.account_id;
    const mailbox = accounts.find((a) => a.id === mailboxId);

    const from = detail?.from?.length ? detail.from : email.from ? [email.from] : [];
    const to = recipientsOf(email, detail);
    const cc = detail?.cc ?? [];
    const bcc = detail?.bcc ?? [];
    const replyTo = (detail?.ReplyTo ?? []).filter((r) => !from.some((f) => sameAddress(f, r)));
    const inReplyTo = (detail?.in_reply_to ?? []).filter(Boolean);
    // The thread row's date is the received time; the Date header only comes with the full fetch.
    const sent = detail ? new Date(detail.date) : null;
    const received = new Date(detail?.internal_date ?? email.date);
    const folder = folderLabel(detail?.folder);
    const subject = email.subject || detail?.subject || "";

    return (
        <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.18, ease: [0.16, 1, 0.3, 1] }}
            className="overflow-hidden"
        >
            <dl
                data-testid="message-details"
                className="mb-3 rounded-md border border-slate-200 bg-slate-50/60 px-3 py-2.5 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-[12px] text-slate-700"
            >
                <Row label="From">
                    <AddressList list={from} />
                </Row>
                {replyTo.length > 0 && (
                    <Row label="Reply to">
                        <AddressList list={replyTo} />
                    </Row>
                )}
                <Row label="To">
                    {to.length > 0 ? <AddressList list={to} /> : <Muted>Undisclosed recipients</Muted>}
                </Row>
                {cc.length > 0 && (
                    <Row label="Cc">
                        <AddressList list={cc} />
                    </Row>
                )}
                {bcc.length > 0 && (
                    <Row label="Bcc">
                        <AddressList list={bcc} />
                    </Row>
                )}
                {sent && (
                    <Row label="Sent">
                        <TimeValue date={sent} />
                    </Row>
                )}
                {(!sent || receivedDiffers(sent, received)) && (
                    <Row label="Received">
                        <TimeValue date={received} />
                    </Row>
                )}
                <Row label="Subject">
                    {subject ? (
                        <Copyable text={subject} what="Subject" className="break-words">
                            {subject}
                        </Copyable>
                    ) : (
                        <Muted>(no subject)</Muted>
                    )}
                </Row>
                {mailbox ? (
                    <Row label="Mailbox">
                        <Copyable text={mailbox.email} what="Address" className="min-w-0">
                            {mailbox.name && mailbox.name !== mailbox.email && (
                                <span className="font-medium text-slate-800">{mailbox.name} </span>
                            )}
                            <span className={mailbox.name && mailbox.name !== mailbox.email ? "text-slate-500" : ""}>
                                {mailbox.email}
                            </span>
                        </Copyable>
                    </Row>
                ) : mailboxId ? (
                    <Row label="Mailbox">
                        <span className="inline-flex items-baseline gap-1.5 flex-wrap min-w-0">
                            <Muted>No longer connected</Muted>
                            <Copyable text={mailboxId} what="Mailbox id" mono>
                                {mailboxId}
                            </Copyable>
                        </span>
                    </Row>
                ) : null}
                {folder && <Row label="Folder">{folder}</Row>}
                {detail ? (
                    <>
                        {detail.message_id && (
                            <Row label="Message-ID">
                                <Copyable text={detail.message_id} what="Message-ID" mono>
                                    {detail.message_id}
                                </Copyable>
                            </Row>
                        )}
                        {inReplyTo.length > 0 && (
                            <Row label="In reply to">
                                <Copyable text={inReplyTo[0]} what="Message-ID" mono>
                                    {inReplyTo[0]}
                                </Copyable>
                            </Row>
                        )}
                        {detail.size > 0 && <Row label="Size">{formatBytes(detail.size)}</Row>}
                    </>
                ) : loading ? (
                    <>
                        <SkeletonRow label="Sent" width="w-[60%]" />
                        <SkeletonRow label="Message-ID" width="w-[70%]" />
                        <SkeletonRow label="Size" width="w-12" />
                    </>
                ) : error ? (
                    <div className="col-span-2 flex items-center gap-1.5 text-[11.5px] text-amber-700">
                        <AlertCircleIcon className="w-3.5 h-3.5 shrink-0" />
                        Couldn't load the sent time, Cc, Message-ID and size for this message.
                    </div>
                ) : null}
            </dl>
        </motion.div>
    );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
    return (
        <>
            <dt className="text-[10px] uppercase tracking-[0.14em] text-slate-400 leading-5 whitespace-nowrap">
                {label}
            </dt>
            <dd className="min-w-0 leading-5">{children}</dd>
        </>
    );
}

function SkeletonRow({ label, width }: { label: string; width: string }) {
    return (
        <Row label={label}>
            <div aria-busy className={cn("h-2.5 mt-1.5 rounded bg-slate-200/70 animate-pulse", width)} />
        </Row>
    );
}

function Muted({ children }: { children: React.ReactNode }) {
    return <span className="text-slate-400">{children}</span>;
}

function TimeValue({ date }: { date: Date }) {
    if (Number.isNaN(date.getTime())) return <Muted>Unknown</Muted>;
    return (
        <span className="inline-flex items-baseline gap-1.5 flex-wrap">
            <span className="tabular-nums">{formatExactTime(date)}</span>
            <span className="text-slate-400">{relativeTime(date)}</span>
        </span>
    );
}

function AddressList({ list }: { list: string[] }) {
    return (
        <ul className="flex flex-wrap gap-x-3 gap-y-0.5 min-w-0">
            {list.map((raw, i) => {
                const { name, address } = addressParts(raw);
                return (
                    <li key={`${address}-${i}`} className="min-w-0 max-w-full">
                        <Copyable text={address} what="Address" className="min-w-0 max-w-full">
                            {name && <span className="font-medium text-slate-800 truncate">{name}</span>}
                            <span className={cn("truncate", name ? "text-slate-500" : "text-slate-800")}>
                                {name ? `<${address}>` : address}
                            </span>
                        </Copyable>
                    </li>
                );
            })}
        </ul>
    );
}

// The icon stays visible on touch, where there is no hover to reveal it.
function Copyable({
    text,
    what,
    mono = false,
    className,
    children,
}: {
    text: string;
    what: string;
    mono?: boolean;
    className?: string;
    children: React.ReactNode;
}) {
    const [copied, setCopied] = React.useState(false);
    const timer = React.useRef<number | null>(null);
    React.useEffect(() => () => {
        if (timer.current) window.clearTimeout(timer.current);
    }, []);

    const copy = async (e: React.MouseEvent) => {
        e.stopPropagation();
        try {
            await navigator.clipboard.writeText(text);
            setCopied(true);
            toast.success(`${what} copied`);
            if (timer.current) window.clearTimeout(timer.current);
            timer.current = window.setTimeout(() => setCopied(false), 1500);
        } catch {
            toast.error("Couldn't copy");
        }
    };

    return (
        <button
            type="button"
            onClick={copy}
            title={`Copy ${what.toLowerCase()}`}
            className={cn(
                "group/copy inline-flex items-center gap-1 max-w-full rounded px-1 -mx-1 text-left transition-colors hover:bg-slate-200/60",
                mono && "font-mono text-[11px] break-all",
                className,
            )}
        >
            {children}
            {copied ? (
                <CheckIcon className="w-3 h-3 shrink-0 text-emerald-600" />
            ) : (
                <CopyIcon className="w-3 h-3 shrink-0 text-slate-400 opacity-60 md:opacity-0 md:group-hover/copy:opacity-100 transition-opacity" />
            )}
        </button>
    );
}
