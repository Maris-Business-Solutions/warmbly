import { execFileSync } from "node:child_process";
import { existsSync, mkdirSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { ensureSession } from "./auth.ts";
import { ARTIFACTS, assertLocal, env, QA_DIR, RUN_DIR, touchStack } from "./env.ts";
import { acquireRecordingLock, releaseRecordingLock } from "./lock.ts";

export type RunInfo = { sha: string; branch: string; dirty: boolean; webURL: string; startedAt: string };

async function reachable(url: string): Promise<boolean> {
  try {
    return (await fetch(url, { signal: AbortSignal.timeout(5000) })).ok;
  } catch {
    return false;
  }
}

function git(...args: string[]): string {
  return execFileSync("git", args, { cwd: QA_DIR, encoding: "utf8" }).trim();
}

// Keeps only this run's traces: earlier runs' output directories belong to processes that have exited.
function pruneOldResults(): void {
  const dir = join(ARTIFACTS, "test-results");
  if (!existsSync(dir)) return;
  for (const name of readdirSync(dir)) {
    const pid = Number(name.replace(/^run-/, ""));
    if (pid === process.pid) continue;
    let alive = false;
    try {
      process.kill(pid, 0);
      alive = true;
    } catch {
      // Exited.
    }
    if (!alive) rmSync(join(dir, name), { recursive: true, force: true });
  }
}

export default async function globalSetup(): Promise<void> {
  if (env.mode === "none") {
    throw new Error("this worktree has no stack running. Start one with `pnpm stack up` (in qa/), or set QA_WEB_URL and QA_API_URL to a local stack you started yourself.");
  }
  assertLocal(env.webURL);
  assertLocal(env.apiURL);

  const down = [];
  if (!(await reachable(env.webURL))) down.push(`dashboard ${env.webURL}`);
  if (!(await reachable(`${env.apiURL}/health`))) down.push(`backend ${env.apiURL}`);
  if (!(await reachable(`${env.mailpitURL}/api/v1/info`))) down.push(`mailpit ${env.mailpitURL}`);
  if (down.length) {
    throw new Error(`not reachable: ${down.join(", ")}. Start this worktree's stack with \`pnpm stack up\` (in qa/), or run \`pnpm doctor\`.`);
  }

  touchStack();
  await acquireRecordingLock(QA_DIR);
  try {
    rmSync(RUN_DIR, { recursive: true, force: true });
    mkdirSync(RUN_DIR, { recursive: true });
    pruneOldResults();
    await ensureSession();
  } catch (err) {
    releaseRecordingLock();
    throw err;
  }

  const run: RunInfo = {
    sha: git("rev-parse", "HEAD"),
    branch: git("rev-parse", "--abbrev-ref", "HEAD"),
    dirty: git("status", "--porcelain").length > 0,
    webURL: env.webURL,
    startedAt: new Date().toISOString(),
  };
  writeFileSync(join(RUN_DIR, "run.json"), JSON.stringify(run, null, 2));
}
