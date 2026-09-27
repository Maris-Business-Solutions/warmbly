// GET /analytics/direct — hand-written mail, as opposed to campaign sends.
//
// Two sources on purpose, because they answer different questions over
// different sets of messages:
//
//   volume   — from the synced mailbox, so it covers everything the mailbox
//              sent (including mail written in Gmail or on a phone) and it
//              covers history from before tracking existed.
//   tracking — from the send records, so it covers only mail sent through
//              Warmbly by a mailbox with tracking switched on, and only since
//              it was switched on.
//
// The UI keeps them in separate cards for that reason. Blending them into one
// "open rate" would divide opens we can see by sends we never measured.

export interface DirectMailVolume {
    sent: number;
    received: number;
    /** Outbound threads whose first message was ours. */
    threads_started: number;
    /** Those that got a message back. */
    replied: number;
    reply_rate: number;
    /** Delivery failures that came back. Never counted as replies. */
    bounced: number;
    /** Median contact turnaround, across answered threads. 0 when none are. */
    median_reply_minutes: number;
}

export interface DirectMailTracking {
    mailboxes_opted_in: number;
    mailboxes_total: number;
    /** The denominator for both rates: untracked sends are not counted here. */
    tracked_sent: number;
    opened: number;
    /** Automated fetches (Apple MPP and friends), excluded from `opened`. */
    machine_opened: number;
    clicked: number;
    open_rate: number;
    click_rate: number;
}

export interface DirectMailDailyStats {
    date: Date;
    sent: number;
    received: number;
}

export interface DirectMailMailboxStats {
    email_account_id: string;
    email: string;
    track_direct_mail: boolean;
    sent: number;
    received: number;
}

export interface DirectMailContact {
    email: string;
    sent: number;
    received: number;
    last_at: Date;
}

export default interface DirectMailAnalytics {
    period: string;
    volume: DirectMailVolume;
    tracking: DirectMailTracking;
    daily_trend: DirectMailDailyStats[];
    mailboxes: DirectMailMailboxStats[];
    top_contacts: DirectMailContact[];
}
