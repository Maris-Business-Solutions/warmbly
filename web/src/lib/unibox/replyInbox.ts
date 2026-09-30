import type Inbox from "@/lib/api/models/app/emails/Inbox";
import { bareEmail } from "@/lib/helper/emailAddress";

type Mailbox = Pick<Inbox, "id" | "email" | "send_as_email" | "reply_to" | "status">;

/** Where a mailbox's mail asks to be answered: its reply-to, else its own address. */
export function answerAddress(mailbox: Pick<Inbox, "email" | "reply_to">): string {
    return mailbox.reply_to?.trim() || mailbox.email;
}

/** Whether a mailbox's reply-to names another mailbox. */
export function repliesGoTo(sender: Mailbox, inbox: Mailbox): boolean {
    const target = bareEmail(sender.reply_to ?? "").trim().toLowerCase();
    if (!target || sender.id === inbox.id) return false;
    return [inbox.email, inbox.send_as_email].some((a) => !!a && a.trim().toLowerCase() === target);
}

/**
 * The mailbox a reply should leave from when the message sits in a shared
 * reply inbox: the one that emailed them and pointed its Reply-To here, so the
 * contact keeps hearing from one address. Null anywhere else.
 */
export function replyInboxSender(
    accounts: Mailbox[],
    holderId: string,
    answersId: string | undefined,
): string | null {
    if (!answersId || answersId === holderId) return null;
    const sender = accounts.find((a) => a.id === answersId);
    const holder = accounts.find((a) => a.id === holderId);
    if (!sender || !holder || sender.status !== "active") return null;
    return repliesGoTo(sender, holder) ? sender.id : null;
}
