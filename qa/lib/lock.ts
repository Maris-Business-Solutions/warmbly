import { closeSync, openSync, readFileSync, unlinkSync, writeSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

// One recording at a time on this machine, whichever worktree or agent starts it:
// each holds a Chromium and an encoder, and parallel ones only starve each other.
const LOCK = join(tmpdir(), "warmbly-qa-recording.lock");
const WAIT_MIN = Number(process.env.QA_LOCK_WAIT_MIN ?? 30);

type Holder = { pid: number; label: string; since: string };

function alive(pid: number): boolean {
  try {
    process.kill(pid, 0);
    return true;
  } catch (err) {
    return (err as NodeJS.ErrnoException).code === "EPERM";
  }
}

function holder(): Holder | undefined {
  try {
    return JSON.parse(readFileSync(LOCK, "utf8")) as Holder;
  } catch {
    return undefined;
  }
}

export async function acquireRecordingLock(label: string): Promise<void> {
  const deadline = Date.now() + WAIT_MIN * 60_000;
  let announced = false;
  for (;;) {
    try {
      const fd = openSync(LOCK, "wx");
      writeSync(fd, JSON.stringify({ pid: process.pid, label, since: new Date().toISOString() } satisfies Holder));
      closeSync(fd);
      return;
    } catch (err) {
      if ((err as NodeJS.ErrnoException).code !== "EEXIST") throw err;
    }
    const h = holder();
    if (!h || !alive(h.pid)) {
      try {
        unlinkSync(LOCK);
      } catch {
        // Another waiter cleared it first.
      }
      continue;
    }
    if (h.pid === process.pid) return;
    if (Date.now() > deadline) throw new Error(`gave up after ${WAIT_MIN} min waiting for the recording in ${h.label} (pid ${h.pid})`);
    if (!announced) {
      console.log(`waiting: another recording is running (${h.label}, pid ${h.pid}, since ${h.since})`);
      announced = true;
    }
    await new Promise((r) => setTimeout(r, 2000));
  }
}

export function releaseRecordingLock(): void {
  if (holder()?.pid !== process.pid) return;
  try {
    unlinkSync(LOCK);
  } catch {
    // Already gone.
  }
}
