import { spawn } from "node:child_process";
import { existsSync, mkdirSync, rmSync, statSync, writeFileSync } from "node:fs";
import { rename, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { Page } from "@playwright/test";
import { VIDEO } from "./env.ts";

// GitHub refuses larger attachments on some plans; above this the clip is re-encoded smaller.
const MAX_BYTES = Number(process.env.QA_MAX_VIDEO_MB ?? 10) * 1024 * 1024;
// x264 holds frames per thread, so this bounds the encoder's memory as much as its CPU.
const THREADS = String(process.env.QA_ENCODE_THREADS ?? 4);

type Frame = { file: string; ts: number };

// Spools Playwright's screencast to disk as full-size JPEGs while the flow runs.
// Encoding waits until the browser is closed (`encodeFrames`), so the two never hold memory at once.
export class Recorder {
  private readonly page: Page;
  private readonly dir: string;
  private readonly frames: Frame[] = [];
  private lowRes = "";

  constructor(page: Page, dir: string) {
    this.page = page;
    this.dir = dir;
  }

  async start(): Promise<void> {
    mkdirSync(this.dir, { recursive: true });
    await this.page.screencast.start({
      size: { width: VIDEO.width, height: VIDEO.height },
      quality: 95,
      onFrame: ({ data, timestamp }) => this.onFrame(data, timestamp),
    });
  }

  // Frames arrive only when the page repaints; their timestamps carry the pauses in between.
  private async onFrame(data: Buffer, timestamp: number): Promise<void> {
    if (!this.frames.length && !(await this.painted())) return;
    if (!this.frames.length) {
      const size = jpegSize(data);
      if (size && size.width < VIDEO.width) this.lowRes = `${size.width}x${size.height}`;
    }
    const file = `f${String(this.frames.length).padStart(6, "0")}.jpg`;
    this.frames.push({ file, ts: timestamp });
    await writeFile(join(this.dir, file), data);
  }

  // The video starts at the first frame with visible text, not on the blank page before the app renders.
  private async painted(): Promise<boolean> {
    if (this.page.url() === "about:blank") return false;
    return this.page
      .evaluate(
        () =>
          performance.getEntriesByType("paint").some((e) => e.name === "first-contentful-paint") &&
          (document.body?.innerText ?? "").trim().length > 0,
      )
      .catch(() => false);
  }

  // Writes the frame list ffmpeg reads later. Returns false when nothing was captured.
  async stop(tailMs = 800): Promise<boolean> {
    await this.page.screencast.stop().catch(() => {});
    if (this.lowRes) {
      throw new Error(`screencast frames arrived at ${this.lowRes}: another screencast (trace screenshots?) started first and set the size`);
    }
    if (!this.frames.length) return false;
    const end = Date.now() + tailMs;
    const lines = ["ffconcat version 1.0"];
    this.frames.forEach((f, i) => {
      const next = this.frames[i + 1]?.ts ?? end;
      lines.push(`file '${f.file}'`, `duration ${Math.max(0.001, (next - f.ts) / 1000).toFixed(4)}`);
    });
    // The concat demuxer ignores the last entry's duration unless the file is listed once more.
    lines.push(`file '${this.frames[this.frames.length - 1].file}'`);
    writeFileSync(join(this.dir, "frames.ffconcat"), `${lines.join("\n")}\n`);
    return true;
  }
}

// Encodes a spooled flow to 1080p H.264, then removes the frames.
export async function encodeFrames(framesDir: string, out: string): Promise<void> {
  const list = join(framesDir, "frames.ffconcat");
  if (!existsSync(list)) throw new Error(`no frame list in ${framesDir}`);
  const { width, height, fps, crf, preset } = VIDEO;
  const vf = [
    `fps=${fps}`,
    `scale=${width}:${height}:force_original_aspect_ratio=decrease:flags=lanczos:in_range=pc:out_range=tv`,
    `pad=${width}:${height}:(ow-iw)/2:(oh-ih)/2:white`,
    "format=yuv420p",
  ].join(",");
  await ffmpeg(["-f", "concat", "-safe", "0", "-i", list, "-vf", vf, ...x264(crf, preset), out]);
  for (let next = crf + 5; statSync(out).size > MAX_BYTES && next <= 38; next += 5) {
    const tmp = out.replace(/\.mp4$/, `.crf${next}.mp4`);
    await ffmpeg(["-i", out, ...x264(next, preset), tmp]);
    await rename(tmp, out);
  }
  rmSync(framesDir, { recursive: true, force: true });
}

function x264(crf: number, preset: string): string[] {
  return [
    ...["-c:v", "libx264", "-preset", preset, "-crf", String(crf), "-threads", THREADS],
    ...["-x264-params", "rc-lookahead=20", "-pix_fmt", "yuv420p", "-color_range", "tv", "-movflags", "+faststart"],
  ];
}

async function ffmpeg(args: string[]): Promise<void> {
  let stderr = "";
  const child = spawn("ffmpeg", ["-hide_banner", "-loglevel", "error", "-y", ...args], { stdio: ["ignore", "ignore", "pipe"] });
  child.stderr.on("data", (d: Buffer) => (stderr += d.toString()));
  const code = await new Promise<number | null>((resolve) => child.on("close", resolve));
  if (code !== 0) throw new Error(`ffmpeg exited ${code}: ${stderr.trim()}`);
}

// Reads the frame size from the JPEG's start-of-frame marker.
function jpegSize(buf: Buffer): { width: number; height: number } | undefined {
  for (let i = 2; i + 9 < buf.length; ) {
    if (buf[i] !== 0xff) return undefined;
    const marker = buf[i + 1];
    if (marker >= 0xc0 && marker <= 0xc2) return { height: buf.readUInt16BE(i + 5), width: buf.readUInt16BE(i + 7) };
    i += 2 + buf.readUInt16BE(i + 2);
  }
  return undefined;
}
