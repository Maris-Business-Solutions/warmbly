import { defineConfig } from "@playwright/test";
import { authFile, env, VIDEO } from "./lib/env.ts";

// Playwright empties its output directory when a run starts, so a run waiting on the recording lock
// would delete the traces of the one recording. Each run gets its own; workers inherit the id.
process.env.QA_RUN_ID ??= String(process.pid);

export default defineConfig({
  testDir: "./flows",
  testMatch: "**/*.flow.ts",
  outputDir: `./.artifacts/test-results/run-${process.env.QA_RUN_ID}`,
  globalSetup: "./lib/global-setup.ts",
  // Encodes the run's videos after the browser has exited, then releases the machine-wide recording lock.
  globalTeardown: "./lib/global-teardown.ts",
  // One flow at a time: a 1080p encode per parallel worker starves the browser and the video stutters.
  workers: 1,
  fullyParallel: false,
  retries: 0,
  timeout: 180_000,
  expect: { timeout: 15_000 },
  reporter: [["list"]],
  use: {
    browserName: "chromium",
    // Full Chromium in new headless mode renders exactly like the desktop browser.
    channel: "chromium",
    baseURL: env.webURL,
    storageState: authFile(),
    viewport: { width: VIDEO.width, height: VIDEO.height },
    deviceScaleFactor: 1,
    colorScheme: "light",
    locale: "en-US",
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    video: "off",
    screenshot: "off",
    // Trace screenshots would start a small screencast first, and the recorder would share its size.
    trace: { mode: "retain-on-failure", screenshots: false, snapshots: true },
    // Human pacing, so a reviewer can follow each click in the video.
    launchOptions: { slowMo: Number(process.env.QA_SLOWMO ?? 250) },
  },
});
