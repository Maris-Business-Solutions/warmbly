// Checks everything a recording needs and prints the fix for whatever is missing.
//   node scripts/doctor.ts
import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import { chromium } from "@playwright/test";
import { authFile, env } from "../lib/env.ts";

let failed = false;

function report(ok: boolean, what: string, fix?: string): void {
  console.log(`${ok ? "\x1b[32m✓\x1b[0m" : "\x1b[31m✗\x1b[0m"} ${what}`);
  if (!ok) {
    failed = true;
    if (fix) console.log(`    fix: ${fix}`);
  }
}

function run(cmd: string, args: string[]): { ok: boolean; out: string } {
  const res = spawnSync(cmd, args, { encoding: "utf8" });
  return { ok: res.status === 0, out: `${res.stdout ?? ""}${res.stderr ?? ""}` };
}

function atLeast(version: string, min: number[]): boolean {
  const parts = version.split(".").map(Number);
  for (let i = 0; i < min.length; i++) {
    if ((parts[i] ?? 0) !== min[i]) return (parts[i] ?? 0) > min[i];
  }
  return true;
}

async function reachable(url: string): Promise<boolean> {
  try {
    return (await fetch(url, { signal: AbortSignal.timeout(4000) })).ok;
  } catch {
    return false;
  }
}

const node = process.versions.node;
report(atLeast(node, [22, 18]), `node ${node} (22.18+ runs the TypeScript scripts directly)`, "install Node 22.18 or newer");

const ffmpeg = run("ffmpeg", ["-hide_banner", "-encoders"]);
report(ffmpeg.ok, "ffmpeg on PATH", "install ffmpeg (pacman -S ffmpeg / apt install ffmpeg / brew install ffmpeg)");
if (ffmpeg.ok) report(/libx264/.test(ffmpeg.out), "ffmpeg has libx264", "install an ffmpeg build with libx264 (the distro package has it)");

const exe = chromium.executablePath();
report(existsSync(exe), "Playwright Chromium installed", "pnpm exec playwright install chromium");
if (existsSync(exe)) {
  try {
    const browser = await chromium.launch({ channel: "chromium" });
    await browser.close();
    report(true, "Chromium launches");
  } catch (err) {
    report(false, `Chromium launches (${String(err).split("\n")[0]})`, "sudo pnpm exec playwright install-deps chromium");
  }
}

const gh = run("gh", ["--version"]);
const ghVersion = gh.out.match(/gh version (\d+\.\d+\.\d+)/)?.[1] ?? "";
report(gh.ok && atLeast(ghVersion, [2, 99]), `gh ${ghVersion || "missing"} (2.99+ uploads media with --attach)`, "install or upgrade the GitHub CLI: https://cli.github.com");
if (gh.ok) report(run("gh", ["auth", "status"]).ok, "gh is signed in", "gh auth login");

console.log("");
const docker = run("docker", ["info", "--format", "{{.ServerVersion}}"]);
report(docker.ok, "docker is running", "start the docker daemon");
if (docker.ok) {
  const names = run("docker", ["ps", "--format", "{{.Names}}"]).out;
  report(/^warmbly-postgres-1$/m.test(names), "shared postgres (warmbly-postgres-1)", "make infra   (from the repo root)");
  const images: [string, string][] = [
    ["redis:7-alpine", "every mode"],
    ["nats:2.10-alpine", "every mode"],
    ["axllent/mailpit:latest", "every mode"],
    ["ghcr.io/warmbly/warmbly/realtime:prod", "full and sandbox"],
    ["dovecot/dovecot:latest", "sandbox"],
    ["ghcr.io/warmbly/warmbly/tracking:prod", "sandbox"],
  ];
  for (const [image, modes] of images) {
    const present = run("docker", ["image", "inspect", image]).ok;
    const required = modes === "every mode" || (modes.includes(env.mode) && env.mode !== "external");
    if (present) report(true, `image ${image}`);
    else if (required) report(false, `image ${image} (${modes})`, `docker pull ${image}`);
    else console.log(`  - image ${image} not pulled (needed for ${modes}): docker pull ${image}`);
  }
}

console.log("");
if (env.mode === "none") {
  report(false, "this worktree's stack is running", "pnpm stack up   (lite; `full` or `sandbox` for more)");
} else {
  console.log(`  stack: ${env.mode === "external" ? "QA_WEB_URL" : `mode ${env.mode}`}, signs in as ${env.email}`);
  report(await reachable(env.webURL), `dashboard ${env.webURL}`, "pnpm stack up");
  report(await reachable(`${env.apiURL}/health`), `backend ${env.apiURL}`, "pnpm stack up");
  report(await reachable(`${env.mailpitURL}/api/v1/info`), `mailpit ${env.mailpitURL} (login codes)`, "pnpm stack up");
  console.log(`  session: ${existsSync(authFile()) ? "saved" : "none yet; the first `pnpm proof` signs in"}`);
}

process.exit(failed ? 1 : 0);
