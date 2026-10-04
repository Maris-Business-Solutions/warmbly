// Human labels for agent tool calls, in two tenses: what the agent is doing
// while the call runs ("Searching contacts") and what it did once it returns
// ("Searched contacts"). Derived from the tool name so a tool added to
// internal/app/aitools reads well without an entry here.

type Tense = { active: string; done: string };

const VERBS: Record<string, Tense> = {
    add: { active: "Adding", done: "Added" },
    bulk: { active: "Editing", done: "Edited" },
    cancel: { active: "Canceling", done: "Canceled" },
    complete: { active: "Completing", done: "Completed" },
    compose: { active: "Composing", done: "Composed" },
    create: { active: "Creating", done: "Created" },
    delete: { active: "Deleting", done: "Deleted" },
    disconnect: { active: "Disconnecting", done: "Disconnected" },
    draft: { active: "Drafting", done: "Drafted" },
    fetch: { active: "Fetching", done: "Fetched" },
    get: { active: "Reading", done: "Read" },
    invite: { active: "Inviting", done: "Invited" },
    list: { active: "Listing", done: "Listed" },
    load: { active: "Loading", done: "Loaded" },
    mark: { active: "Marking", done: "Marked" },
    mint: { active: "Creating", done: "Created" },
    move: { active: "Moving", done: "Moved" },
    preview: { active: "Previewing", done: "Previewed" },
    remove: { active: "Removing", done: "Removed" },
    revoke: { active: "Revoking", done: "Revoked" },
    rotate: { active: "Rotating", done: "Rotated" },
    run: { active: "Running", done: "Ran" },
    search: { active: "Searching", done: "Searched" },
    send: { active: "Sending", done: "Sent" },
    set: { active: "Updating", done: "Updated" },
    snooze: { active: "Snoozing", done: "Snoozed" },
    submit: { active: "Submitting", done: "Submitted" },
    unsnooze: { active: "Unsnoozing", done: "Unsnoozed" },
    update: { active: "Updating", done: "Updated" },
    verify: { active: "Verifying", done: "Verified" },
};

// Whole-name overrides where the verb + object reading would be clumsy.
const NAMED: Record<string, Tense> = {
    search_web: { active: "Searching the web", done: "Searched the web" },
    fetch_url: { active: "Reading a web page", done: "Read a web page" },
    load_skill: { active: "Loading a skill", done: "Loaded a skill" },
    get_thread: { active: "Reading the thread", done: "Read the thread" },
    list_threads: { active: "Scanning the inbox", done: "Scanned the inbox" },
    get_dashboard_analytics: { active: "Reading analytics", done: "Read analytics" },
    get_advisor_summary: { active: "Checking the advisor", done: "Checked the advisor" },
    bulk_edit_contacts: { active: "Editing contacts", done: "Edited contacts" },
    add_tag: { active: "Adding a label", done: "Added a label" },
    remove_tag: { active: "Removing a label", done: "Removed a label" },
    set_thread_labels: { active: "Labeling the thread", done: "Labeled the thread" },
};

export function toolTense(tool: string): Tense {
    const named = NAMED[tool];
    if (named) return named;
    const [verb, ...rest] = tool.split("_");
    const object = rest.join(" ");
    const v = VERBS[verb];
    if (!v) {
        const plain = tool.replace(/_/g, " ");
        const cap = plain.charAt(0).toUpperCase() + plain.slice(1);
        return { active: cap, done: cap };
    }
    return {
        active: object ? `${v.active} ${object}` : v.active,
        done: object ? `${v.done} ${object}` : v.done,
    };
}

export function toolLabel(tool: string, done = true): string {
    const t = toolTense(tool);
    return done ? t.done : t.active;
}

// The imperative form an approval asks about ("Update contact fields").
export function toolAction(tool: string): string {
    const plain = tool.replace(/_/g, " ");
    return plain.charAt(0).toUpperCase() + plain.slice(1);
}
