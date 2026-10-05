import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";
import { apps, deploymentMatrix } from "./pages-projects.mjs";

test("one list expands every supported app with its actual build script and output", () => {
  const targets = Object.keys(apps).map((app) => ({ app, project: `warmbly-${app}` }));
  const matrix = deploymentMatrix(JSON.stringify(targets));
  assert.equal(matrix.include.length, 4);
  for (const entry of matrix.include) {
    assert.deepEqual(entry, { ...targets.find((target) => target.app === entry.app), ...apps[entry.app] });
    const pkg = JSON.parse(readFileSync(new URL(`../../${entry.directory}/package.json`, import.meta.url), "utf8"));
    assert.equal(typeof pkg.scripts[entry.build_script], "string");
    if (entry.app === "web" || entry.app === "admin") assert.ok(pkg.scripts[entry.build_script].includes("WARMBLY_CONFIG_OUT=dist/config.js"));
  }
  assert.equal(apps.docs.output_dir, "docs/out");
  assert.match(readFileSync(new URL("../../docs/next.config.mjs", import.meta.url), "utf8"), /output:\s*['"]export['"]/);
});

test("supports more than two projects, including multiple deployments of the same app", () => {
  const targets = Array.from({ length: 6 }, (_, i) => ({ app: "web", project: `dashboard-${i}` }));
  assert.equal(deploymentMatrix(JSON.stringify(targets)).include.length, 6);
  assert.equal(deploymentMatrix(JSON.stringify(targets.slice(0, 2))).include.length, 2);
});

test("keeps the existing single-dashboard variable compatible and gives the list precedence", () => {
  assert.equal(deploymentMatrix("", "existing-dashboard").include[0].project, "existing-dashboard");
  assert.equal(deploymentMatrix('[{"app":"docs","project":"docs-project"}]', "old-dashboard").include[0].app, "docs");
});

test("rejects invalid config, unknown apps, duplicate projects and build/path overrides", () => {
  for (const input of ["not json", "{}", "[]", "null", "[null]", '[{"app":"worker","project":"test"}]', '[{"app":"__proto__","project":"test"}]', '[{"app":"web","project":"bad/name"}]', '[{"app":"web","project":"bad\nname"}]', '[{"app":"web","project":"ok","directory":"../"}]', '[{"app":"web","project":"same"},{"app":"admin","project":"same"}]']) {
    assert.throws(() => deploymentMatrix(input));
  }
  assert.throws(() => deploymentMatrix("", ""));
  assert.throws(() => deploymentMatrix('[{"app":["web"],"project":"ok"}]'));
  assert.throws(() => deploymentMatrix(JSON.stringify(Array.from({ length: 257 }, (_, i) => ({ app: "web", project: `p-${i}` })))));
  assert.equal(deploymentMatrix(JSON.stringify(Array.from({ length: 256 }, (_, i) => ({ app: "web", project: `p-${i}` })))).include.length, 256);
});

test("CLI emits a safe matrix for the workflow and does not expose invalid input", () => {
  const work = mkdtempSync(join(tmpdir(), "pages-matrix-"));
  try {
    const output = join(work, "output");
    const env = { ...process.env, GITHUB_OUTPUT: output, CLOUDFLARE_PAGES_PROJECTS: '[{"app":"admin","project":"admin-project"}]' };
    const script = fileURLToPath(new URL("./pages-projects.mjs", import.meta.url));
    const result = spawnSync(process.execPath, [script], { env, encoding: "utf8" });
    assert.equal(result.status, 0, result.stderr);
    const content = readFileSync(output, "utf8");
    assert.equal(content.split("\n").length, 2);
    assert.equal(JSON.parse(content.slice("matrix=".length)).include[0].directory, "admin");
    env.CLOUDFLARE_PAGES_PROJECTS = "private-invalid-input";
    const invalid = spawnSync(process.execPath, [script], { env, encoding: "utf8" });
    assert.equal(invalid.status, 1);
    assert.ok(!invalid.stderr.includes("private-invalid-input"));
    assert.equal(readFileSync(output, "utf8"), content);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});
