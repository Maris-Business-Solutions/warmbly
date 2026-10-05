// Shared definitions for the StateLegend component. One source of truth for
// what each enum state actually means, so every page explains the same term
// the same way. Tones mirror the badge tone maps used by the tables.

import type { LegendEntry } from "@/components/StateLegend";
import { TONE } from "@/lib/tones";

export const WORKER_HEALTH_LEGEND: LegendEntry[] = [
    {
        term: "healthy",
        tone: TONE.success,
        description: "Accepting new mailbox assignments normally.",
    },
    {
        term: "watch",
        tone: TONE.warning,
        description: "Deliverability signals trending down. Placement is deprioritized.",
    },
    {
        term: "throttled",
        tone: TONE.orange,
        description: "Receives fewer new assignments while its mailboxes recover.",
    },
    {
        term: "quarantined",
        tone: TONE.danger,
        description: "No new assignments. Existing mailboxes should be migrated off.",
    },
    {
        term: "blocked",
        tone: TONE.danger,
        description: "Excluded from placement entirely until an admin clears it.",
    },
];

// Mailbox health drives warmup pool membership and campaign throttling.
export const MAILBOX_HEALTH_LEGEND: LegendEntry[] = [
    {
        term: "healthy",
        tone: TONE.success,
        description: "Normal sending and warmup. Only healthy mailboxes are picked for pools.",
    },
    {
        term: "watch",
        tone: TONE.warning,
        description:
            "Early warning signals (spam placement or complaints trending up). Volume lowered, monitoring increased.",
    },
    {
        term: "throttled",
        tone: TONE.orange,
        description: "Sending slowed after repeated warnings while signals recover.",
    },
    {
        term: "quarantined",
        tone: TONE.danger,
        description:
            "Removed from the shared warmup pool for a cooldown after crossing the quarantine band.",
    },
    {
        term: "blocked",
        tone: TONE.danger,
        description:
            "Barred from warmup until the cooldown expires and re-entry checks pass. Requires review.",
    },
];

// Offline warmup-content generation job lifecycle.
export const GENERATION_JOB_LEGEND: LegendEntry[] = [
    {
        term: "pending",
        tone: TONE.warning,
        description: "Queued and waiting to start.",
    },
    {
        term: "running",
        tone: TONE.info,
        description: "Generating threads right now. Counts update as it goes.",
    },
    {
        term: "completed",
        tone: TONE.success,
        description:
            "Finished. Check generated vs lint-rejected counts; rejected threads never enter the library.",
    },
    {
        term: "failed",
        tone: TONE.danger,
        description: "Produced no usable threads. The error column has the reason.",
    },
    {
        term: "cancelled",
        tone: TONE.neutral,
        description: "Stopped by an admin before it finished.",
    },
];
