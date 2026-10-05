// Shared constants + pure helpers for the warmup-content section, so every
// tab renders the same tones and date formatting.
//
// Keep this file JSX-free: the badge components live in `components.tsx` so
// React Fast Refresh stays happy.

import type { Tone } from "@/lib/tones";

const JOB_STATUS_TONE: Record<string, Tone> = {
    pending: "warning",
    queued: "warning",
    running: "info", // matches GENERATION_JOB_LEGEND
    completed: "success",
    succeeded: "success",
    failed: "danger",
    error: "danger",
    cancelled: "neutral",
    canceled: "neutral",
};

export function jobTone(status: string): Tone {
    return JOB_STATUS_TONE[status] ?? "neutral";
}

// OpenAI Batch lifecycle: in flight reads warning/info, terminal-good success,
// terminal-bad danger, cancelled neutral.
const BATCH_STATUS_TONE: Record<string, Tone> = {
    validating: "warning",
    in_progress: "info",
    finalizing: "info",
    cancelling: "warning",
    completed: "success",
    failed: "danger",
    expired: "danger",
    cancelled: "neutral",
};

export function batchTone(status: string): Tone {
    return BATCH_STATUS_TONE[status] ?? "neutral";
}

const CONTENT_STATUS_TONE: Record<string, Tone> = {
    active: "success",
    archived: "neutral",
    draft: "warning",
};

export function contentTone(status: string): Tone {
    return CONTENT_STATUS_TONE[status] ?? "neutral";
}

export function fmtDate(s: string | null | undefined): string {
    if (!s) return "—";
    return new Date(s).toLocaleString();
}
