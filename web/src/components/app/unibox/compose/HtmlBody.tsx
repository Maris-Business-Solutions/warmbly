// A composer body in HTML mode, and the switch in and out of it. The editing
// surface is the mailbox signature editor: visual, source, preview, plain text.

import React from "react";
import { CodeXmlIcon } from "lucide-react";
import EmailEditor from "@/components/app/EmailEditor";
import { useConfirm } from "@/hooks/context/confirm";
import { htmlHasContent, htmlToPlain, toHtmlMode, toPlainMode } from "@/lib/email/composerBody";
import type { ComposerBodyState } from "@/lib/email/useComposerBody";
import { cn } from "@/lib/utils";

export function HtmlBody({
    id,
    state,
    onSend,
    onEscape,
    className,
}: {
    id: string;
    state: ComposerBodyState;
    onSend: () => void;
    onEscape?: () => void;
    className?: string;
}) {
    const [code, setCode] = React.useState(false);
    if (state.html === null) return null;
    return (
        <div
            className={cn("min-h-0 overflow-y-auto", className)}
            onKeyDown={(e) => {
                if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                    e.preventDefault();
                    onSend();
                } else if (e.key === "Escape" && document.querySelector('[data-floating], [role="alertdialog"]')) {
                    // A link or image popover is the innermost layer; the composer stays.
                    e.stopPropagation();
                } else if (e.key === "Escape" && onEscape) {
                    e.preventDefault();
                    e.stopPropagation();
                    onEscape();
                }
            }}
        >
            <EmailEditor
                id={id}
                kind="email"
                htmlText={state.html}
                setHtmlText={state.setHtml}
                plainText={state.body}
                setPlainText={state.setBody}
                sync={state.sync}
                setSync={state.setSync}
                code={code}
                setCode={setCode}
                toPlain={htmlToPlain}
            />
        </div>
    );
}

export function HtmlModeToggle({ state, disabled }: { state: ComposerBodyState; disabled?: boolean }) {
    const confirm = useConfirm();
    const on = state.html !== null;
    const toggle = () => {
        if (!on) {
            state.setValue(toHtmlMode);
            return;
        }
        if (!htmlHasContent(state.html ?? "")) {
            state.setValue(toPlainMode);
            return;
        }
        confirm.show(
            "Send this email as plain text? Formatting, images and links behind words are removed, and the plain-text version becomes the body.",
            () => state.setValue(toPlainMode),
        );
    };
    return (
        <button
            type="button"
            onClick={toggle}
            disabled={disabled}
            aria-pressed={on}
            title={
                disabled
                    ? "Keep or discard the AI draft first"
                    : on
                      ? "Switch back to plain text"
                      : "Write this email in HTML"
            }
            className={cn(
                "h-7 px-2 rounded-md border text-[12px] inline-flex items-center gap-1 transition-colors disabled:opacity-50 disabled:cursor-not-allowed",
                on
                    ? "border-sky-200 bg-sky-50 text-sky-700 hover:border-sky-300"
                    : "border-slate-200 hover:border-slate-300 text-slate-700 hover:text-slate-900",
            )}
        >
            <CodeXmlIcon className="w-3 h-3" />
            HTML
        </button>
    );
}
