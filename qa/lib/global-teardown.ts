import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { RUN_DIR } from "./env.ts";
import { releaseRecordingLock } from "./lock.ts";
import type { FlowMeta } from "./proof.ts";
import { encodeFrames } from "./recorder.ts";

// Runs after the workers, and their browser, have exited: encodes each flow's frames one at a time.
export default async function globalTeardown(): Promise<void> {
  try {
    if (!existsSync(RUN_DIR)) return;
    for (const entry of readdirSync(RUN_DIR, { withFileTypes: true })) {
      const metaFile = join(RUN_DIR, entry.name, "meta.json");
      if (!entry.isDirectory() || !existsSync(metaFile)) continue;
      const meta = JSON.parse(readFileSync(metaFile, "utf8")) as FlowMeta;
      if (!meta.frames) continue;
      const out = join(RUN_DIR, entry.name, "video.mp4");
      const started = Date.now();
      await encodeFrames(meta.frames, out);
      meta.video = out;
      meta.frames = undefined;
      writeFileSync(metaFile, JSON.stringify(meta, null, 2));
      console.log(`encoded ${entry.name}/video.mp4 in ${((Date.now() - started) / 1000).toFixed(1)}s`);
    }
  } finally {
    releaseRecordingLock();
  }
}
