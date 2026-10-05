// Non-component helpers the Sends tabs share.

import type { Tone } from "@/lib/tones";

export const TASK_TONE: Record<string, Tone> = {
    pending: "warning",
    processing: "warning",
    completed: "success",
    failed: "danger",
};

export function fmt(n: number | undefined): string {
    return n === undefined ? "—" : n.toLocaleString();
}
