// GET /analytics/inbox-tagging, the review surface.
//
// Automatic tagging writes labels and a relevance score, and, once a workspace
// switches an action on, may hold, stop, open a task for or suppress the
// sender. Every row shows what was decided, how sure it was, and what it did,
// so a person can watch the labels be right before letting them act.

export interface InboxTagRow {
    id: string;
    message_id: string;
    thread_id: string;
    /** What the message is. One of the eight kinds in the taxonomy. */
    kind: string;
    kind_confidence: number;
    /** "header" when an offline deterministic rule decided, "model" when Jev did. */
    kind_source: string;
    /** Only set for a human reply; meaningless on a bounce. */
    intent: string;
    intent_confidence: number;
    relevance: number;
    priority: string;
    /** True when a choice came back below the confidence floor. */
    needs_review: boolean;
    /** Which confidence fell below the floor: kind or intent. */
    review_reason: "" | "kind" | "intent";
    labels: string[];
    /** What the workspace's switches let this verdict do: hold, stop, task, suppress. */
    actions: string[];
    /** Every raw probability, exactly as the API returned it. */
    answers: Record<string, unknown>;
    /**
     * The out-of-office return date (YYYY-MM-DD) the model was asked to
     * confirm; its answer is `answers.return_date.noul`. Null when not asked.
     */
    return_date: string | null;
    model: string;
    input_tokens: number;
    created_at: Date;
}

export default interface InboxTagReview {
    /** False when the instance has no key or the switch is off. */
    enabled: boolean;
    data: InboxTagRow[];
    total: number;
    summary: {
        total: number;
        needs_review: number;
        from_offline: number;
        /** Verdicts that held, stopped, opened a task or suppressed. */
        acted: number;
    };
    pagination: {
        next_cursor: string | null;
        has_more: boolean;
    };
}
