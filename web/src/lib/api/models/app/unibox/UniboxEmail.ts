import type { UniboxFolder } from "./UniboxSearch";

export default interface UniboxEmail {
  id: string;
  from: string;
  to: string;
  /** Every To recipient as the header carried them; `to` is the first. */
  recipients?: string[];
  subject: string;
  date: Date;
  is_seen: boolean;
  thread_id?: string;
  account_id: string;
  /** One-line preview from the message list; the full body is fetched per message. */
  snippet?: string;
  /** Number of messages in the conversation (for the stacked count badge). */
  message_count?: number;
  /** Conversation labels (categories) assigned to the thread. */
  labels?: { id: string; title: string; color: string }[];
  /** Thread reads: the workspace mailbox that sent the email this one replies to, when another mailbox holds it. */
  answers_mailbox_id?: string;
}

/**
 * GET /unibox/:id — the full message: envelope, body, and where it lives.
 * Addresses are arrays here where the list shape carries one string.
 */
export interface UniboxEmailDetail {
  id: string;
  /** The connected mailbox the message belongs to. */
  email_id: string;
  thread_id: string;
  parent_id: string;
  /** RFC Message-ID header, angle brackets included. */
  message_id: string;
  in_reply_to: string[];
  from: string[];
  to: string[];
  cc: string[];
  bcc: string[];
  /** Reply-To addresses; the API serializes the key as `ReplyTo`. */
  ReplyTo: string[];
  subject: string;
  /** From the message's own Date header. */
  date: Date;
  /** When the mailbox received it, as the provider reports. */
  internal_date: Date;
  /** RFC822 size in bytes; 0 when the provider did not report one. */
  size: number;
  flags: string[];
  folder: UniboxFolder;
  /** Sanitized by the API before it is sent; safe to render. */
  body_html: string;
  body_plain: string;
  /** True when the stored body could not be read and body_plain is only the preview. */
  body_truncated?: boolean;
}
