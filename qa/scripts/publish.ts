// Publishes the last recorded run to the current branch's pull request.
// First publish: a Proof section appended to the PR description (stills inline, walkthrough videos last).
// Every later publish: a comment with before/after pairs for the stills that changed since the previous one.
//
//   node scripts/publish.ts                 publish
//   node scripts/publish.ts --dry-run       write the body and print the gh command, upload nothing
//   node scripts/publish.ts --no-video      stills only
//   node scripts/publish.ts --force         comment even when no still changed
//   node scripts/publish.ts --replace-body  rewrite the PR description's Proof section instead of commenting
//   node scripts/publish.ts --allow-failed  publish a run with failing flows
import { execFileSync, spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { join, relative } from "node:path";
import pixelmatch from "pixelmatch";
import pngjs from "pngjs";
import { ARTIFACTS, assertLocal, QA_DIR, RUN_DIR } from "../lib/env.ts";
import type { RunInfo } from "../lib/global-setup.ts";
import type { Box, FlowMeta, Shot } from "../lib/proof.ts";

const { PNG } = pngjs;
const MARK = "<!-- warmbly-proof:start -->";
const MARK_END = "<!-- warmbly-proof:end -->";
const MAX_ATTACHMENTS = 50;

const flags = new Set(process.argv.slice(2));
for (const f of flags) {
  if (!["--dry-run", "--no-video", "--force", "--replace-body", "--allow-failed"].includes(f)) die(`unknown flag ${f}`);
}

function die(msg: string): never {
  console.error(`publish: ${msg}`);
  process.exit(1);
}

function git(...args: string[]): string {
  return execFileSync("git", args, { cwd: QA_DIR, encoding: "utf8" }).trim();
}

function short(sha: string): string {
  return sha.slice(0, 9);
}

type Baseline = { sha: string; shots: Record<string, string>; ignore?: Record<string, Box[]> };
type Pr = { number: number; body: string; url: string };
type Change = { flow: FlowMeta; shot: Shot; key: string; kind: "new" | "changed"; before?: string };

function loadRun(): { run: RunInfo; flows: FlowMeta[] } {
  const runFile = join(RUN_DIR, "run.json");
  if (!existsSync(runFile)) die("no recorded run; record one with `pnpm proof` first");
  const run = JSON.parse(readFileSync(runFile, "utf8")) as RunInfo;
  const flows = readdirSync(RUN_DIR, { withFileTypes: true })
    .filter((d) => d.isDirectory() && existsSync(join(RUN_DIR, d.name, "meta.json")))
    .map((d) => JSON.parse(readFileSync(join(RUN_DIR, d.name, "meta.json"), "utf8")) as FlowMeta)
    .sort((a, b) => a.slug.localeCompare(b.slug));
  if (!flows.length) die("the last run recorded no flows");
  return { run, flows };
}

function currentPr(): Pr {
  const res = spawnSync("gh", ["pr", "view", "--json", "number,body,url"], { cwd: QA_DIR, encoding: "utf8" });
  // A dry run previews the first publish before the PR exists.
  if (res.status !== 0 && flags.has("--dry-run")) return { number: 0, body: "", url: "(no pull request yet)" };
  if (res.status !== 0) die(`no pull request for this branch yet. Open it with \`gh pr create\`, then publish.\n${res.stderr.trim()}`);
  return JSON.parse(res.stdout) as Pr;
}

// Blanks the regions a flow marked as changing on its own, in both images, before comparing.
function blank(img: InstanceType<typeof PNG>, boxes: Box[]): void {
  for (const b of boxes) {
    for (let y = Math.max(0, b.y); y < Math.min(img.height, b.y + b.height); y++) {
      img.data.fill(0, (y * img.width + Math.max(0, b.x)) * 4, (y * img.width + Math.min(img.width, b.x + b.width)) * 4);
    }
  }
}

function differs(a: string, b: string, ignore: Box[]): boolean {
  const x = PNG.sync.read(readFileSync(a));
  const y = PNG.sync.read(readFileSync(b));
  if (x.width !== y.width || x.height !== y.height) return true;
  blank(x, ignore);
  blank(y, ignore);
  const changed = pixelmatch(x.data, y.data, undefined, x.width, x.height, { threshold: 0.1 });
  return changed > Math.max(40, x.width * x.height * 0.0002);
}

function keyOf(flow: FlowMeta, shot: Shot): string {
  return `${flow.slug}__${shot.name}`;
}

// Attachments are referenced in the body by the exact path passed to --attach, which gh rewrites to the upload.
class Attachments {
  readonly dir = join(ARTIFACTS, "publish");
  readonly paths: string[] = [];

  constructor() {
    rmSync(this.dir, { recursive: true, force: true });
    mkdirSync(this.dir, { recursive: true });
  }

  add(src: string, name: string): string {
    const dest = join(this.dir, name);
    copyFileSync(src, dest);
    const ref = `./${relative(QA_DIR, dest)}`;
    this.paths.push(ref);
    return ref;
  }
}

function sentence(title: string): string {
  return title.charAt(0).toUpperCase() + title.slice(1);
}

function image(alt: string, ref: string): string {
  return `![${alt.replace(/[[\]]/g, "")}](${ref})`;
}

function initialSection(run: RunInfo, flows: FlowMeta[], files: Attachments, videos: FlowMeta[]): string {
  const lines = [
    MARK,
    "## Proof",
    "",
    `Recorded at \`${short(run.sha)}\` against a local stack with seed data, 1920x1080.`,
    "",
  ];
  for (const flow of flows) {
    lines.push(`### ${sentence(flow.title)}`, "");
    for (const shot of flow.shots) lines.push(`**${shot.caption}**`, "", image(shot.caption, files.add(shot.file, `${flow.slug}-${shot.name}.png`)), "");
  }
  if (videos.length) {
    lines.push("### Walkthrough", "", "Videos below, in this order:", "");
    videos.forEach((f, i) => lines.push(`${i + 1}. ${sentence(f.title)}`));
    lines.push("");
  }
  lines.push(MARK_END);
  return lines.join("\n");
}

function updateComment(run: RunInfo, base: Baseline | undefined, changes: Change[], unchanged: number, videos: FlowMeta[], files: Attachments): string {
  const lines: string[] = ["<!-- warmbly-proof:update -->"];
  if (base) {
    lines.push(`### Proof update: \`${short(base.sha)}..${short(run.sha)}\``, "");
    try {
      const subjects = git("log", "--format=%s", "--max-count=20", `${base.sha}..${run.sha}`).split("\n").filter(Boolean);
      for (const s of subjects) lines.push(`- ${s}`);
      if (subjects.length) lines.push("");
    } catch {
      // A rebase can drop the old commit; the range header still says what moved.
    }
  } else {
    lines.push(`### Proof update at \`${short(run.sha)}\``, "", "No earlier publish is recorded on this machine, so every screen is shown.", "");
  }

  let flowTitle = "";
  for (const c of changes) {
    if (c.flow.title !== flowTitle) {
      flowTitle = c.flow.title;
      lines.push(`#### ${sentence(flowTitle)}`, "");
    }
    const after = files.add(c.shot.file, `${c.flow.slug}-${c.shot.name}.png`);
    if (c.kind === "changed" && c.before) {
      const before = files.add(c.before, `before-${c.flow.slug}-${c.shot.name}.png`);
      lines.push(`**${c.shot.caption}** (changed)`, "", "| Before | After |", "| --- | --- |");
      lines.push(`| ${image(`${c.shot.caption}, before`, before)} | ${image(`${c.shot.caption}, after`, after)} |`, "");
    } else {
      lines.push(`**${c.shot.caption}** (new)`, "", image(c.shot.caption, after), "");
    }
  }
  if (unchanged) lines.push(`Unchanged: ${unchanged} screen${unchanged === 1 ? "" : "s"}.`, "");
  if (videos.length) {
    lines.push("Walkthrough videos below, in this order:", "");
    videos.forEach((f, i) => lines.push(`${i + 1}. ${sentence(f.title)}`));
  }
  return lines.join("\n").trimEnd();
}

function saveBaseline(file: string, run: RunInfo, flows: FlowMeta[]): void {
  const dir = join(file, "..", "shots");
  rmSync(dir, { recursive: true, force: true });
  mkdirSync(dir, { recursive: true });
  const shots: Record<string, string> = {};
  const ignore: Record<string, Box[]> = {};
  for (const flow of flows) {
    for (const shot of flow.shots) {
      const key = keyOf(flow, shot);
      shots[key] = join(dir, `${key}.png`);
      copyFileSync(shot.file, shots[key]);
      if (shot.ignore) ignore[key] = shot.ignore;
    }
  }
  writeFileSync(file, JSON.stringify({ sha: run.sha, shots, ignore } satisfies Baseline, null, 2));
}

function main(): void {
  const { run, flows } = loadRun();
  assertLocal(run.webURL);

  const failed = flows.filter((f) => f.status !== "passed");
  if (failed.length && !flags.has("--allow-failed")) {
    die(`flows failed, fix them before publishing: ${failed.map((f) => f.title).join(", ")}`);
  }
  const unencoded = flows.filter((f) => f.frames);
  if (unencoded.length) die(`the run ended before its videos were encoded (${unencoded.map((f) => f.title).join(", ")}); record again`);
  const head = git("rev-parse", "HEAD");
  if (run.sha !== head) die(`the recording is from ${short(run.sha)} but HEAD is ${short(head)}; record again`);
  if (run.dirty) console.warn("publish: warning: the recording was made with uncommitted changes in the tree");
  if (!git("branch", "-r", "--contains", head)) console.warn("publish: warning: HEAD is not pushed yet, so reviewers cannot see this commit");

  const pr = currentPr();
  const baselineFile = join(ARTIFACTS, "published", `pr-${pr.number}`, "state.json");
  const base = existsSync(baselineFile) ? (JSON.parse(readFileSync(baselineFile, "utf8")) as Baseline) : undefined;
  const files = new Attachments();
  const withVideo = (list: FlowMeta[]) => (flags.has("--no-video") ? [] : list.filter((f) => f.video && existsSync(f.video)));
  const bodyFile = join(files.dir, "body.md");
  let args: string[];

  const firstPublish = !pr.body?.includes(MARK);
  if (firstPublish || flags.has("--replace-body")) {
    const videos = withVideo(flows);
    const kept = (pr.body ?? "").split(MARK)[0].trimEnd();
    writeFileSync(bodyFile, `${kept ? `${kept}\n\n` : ""}${initialSection(run, flows, files, videos)}\n`);
    for (const f of videos) files.add(f.video!, `${f.slug}.mp4`);
    args = ["pr", "edit", String(pr.number), "--body-file", bodyFile];
  } else {
    const changes: Change[] = [];
    let unchanged = 0;
    for (const flow of flows) {
      for (const shot of flow.shots) {
        const key = keyOf(flow, shot);
        const before = base?.shots[key];
        if (!before || !existsSync(before)) changes.push({ flow, shot, key, kind: "new" });
        else if (differs(before, shot.file, [...(base?.ignore?.[key] ?? []), ...(shot.ignore ?? [])])) changes.push({ flow, shot, key, kind: "changed", before });
        else unchanged++;
      }
    }
    if (!changes.length && !flags.has("--force")) {
      console.log(`publish: no screen changed since ${base ? short(base.sha) : "the last publish"}; nothing posted (--force posts anyway)`);
      return;
    }
    const changedFlows = new Set(changes.map((c) => c.flow.slug));
    const videos = withVideo(flows.filter((f) => changedFlows.has(f.slug) || flags.has("--force")));
    writeFileSync(bodyFile, `${updateComment(run, base, changes, unchanged, videos, files)}\n`);
    for (const f of videos) files.add(f.video!, `${f.slug}.mp4`);
    args = ["pr", "comment", String(pr.number), "--body-file", bodyFile];
  }

  if (files.paths.length > MAX_ATTACHMENTS) {
    die(`${files.paths.length} attachments is over gh's limit of ${MAX_ATTACHMENTS}; record fewer flows or pass --no-video`);
  }
  for (const p of files.paths) args.push("--attach", p);

  if (flags.has("--dry-run")) {
    console.log(readFileSync(bodyFile, "utf8"));
    console.log(`\n(dry run) gh ${args.join(" ")}`);
    return;
  }
  const res = spawnSync("gh", args, { cwd: QA_DIR, stdio: "inherit" });
  if (res.status !== 0) die("gh failed; nothing was recorded as published");
  saveBaseline(baselineFile, run, flows);
  console.log(`publish: ${firstPublish || flags.has("--replace-body") ? "description updated" : "comment posted"} on ${pr.url}`);
}

main();
