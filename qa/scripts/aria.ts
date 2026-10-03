// Prints a page's accessibility tree with the saved session: the cheap way to find selectors for a flow.
//   node scripts/aria.ts /app/contacts [css-selector]
import { chromium } from "@playwright/test";
import { existsSync } from "node:fs";
import { assertLocal, authFile, env, QA_DIR, touchStack } from "../lib/env.ts";
import { acquireRecordingLock, releaseRecordingLock } from "../lib/lock.ts";

const [path = "/app/emails", selector = "body"] = process.argv.slice(2);
if (env.mode === "none") {
  console.error("this worktree has no stack running; start one with `pnpm stack up`");
  process.exit(1);
}
assertLocal(env.webURL);
if (!existsSync(authFile())) {
  console.error("no saved session yet; run any flow once (`pnpm proof`) so global setup signs in");
  process.exit(1);
}
touchStack();
await acquireRecordingLock(`${QA_DIR} (aria)`);
const browser = await chromium.launch({ channel: "chromium" });
const context = await browser.newContext({ storageState: authFile(), viewport: { width: 1920, height: 1080 } });
const page = await context.newPage();
await page.goto(new URL(path, env.webURL).toString());
await page.waitForLoadState("networkidle");
console.log(await page.locator(selector).first().ariaSnapshot());
await browser.close();
releaseRecordingLock();
