import { appendFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

export const apps = {
  web: { directory: "web", build_script: "build:pages", output_dir: "web/dist", environment: "dashboard-production", sentry_project_var: "SENTRY_PROJECT_WEB" },
  admin: { directory: "admin", build_script: "build:pages", output_dir: "admin/dist", environment: "admin-production", sentry_project_var: "SENTRY_PROJECT_ADMIN" },
  site: { directory: "site", build_script: "build", output_dir: "site/dist", environment: "site-production", sentry_project_var: "SENTRY_PROJECT_SITE" },
  docs: { directory: "docs", build_script: "build", output_dir: "docs/out", environment: "docs-production", sentry_project_var: "SENTRY_PROJECT_DOCS" },
};

export function deploymentMatrix(projects, legacyProject) {
  const targets = projects?.trim()
    ? JSON.parse(projects)
    : legacyProject?.trim() ? [{ app: "web", project: legacyProject }] : [];
  if (!Array.isArray(targets) || targets.length === 0 || targets.length > 256) {
    throw new Error("Set CLOUDFLARE_PAGES_PROJECTS to a JSON array with 1 to 256 app/project entries.");
  }
  const seen = new Set();
  return { include: targets.map((target) => {
    if (!target || typeof target.app !== "string" || !Object.hasOwn(apps, target.app) || typeof target.project !== "string" || !/^[a-z0-9][a-z0-9-]*$/.test(target.project)) {
      throw new Error("Each Pages target needs an app (web, admin, site, docs) and a valid Pages project name.");
    }
    if (Object.keys(target).some((key) => !["app", "project"].includes(key))) {
      throw new Error("Pages targets accept only app and project fields.");
    }
    if (seen.has(target.project)) throw new Error("Each Pages project must appear only once in the deployment list.");
    seen.add(target.project);
    return { ...target, ...apps[target.app] };
  }) };
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  try {
    const matrix = deploymentMatrix(process.env.CLOUDFLARE_PAGES_PROJECTS, process.env.CLOUDFLARE_PAGES_PROJECT_NAME);
    appendFileSync(process.env.GITHUB_OUTPUT, `matrix=${JSON.stringify(matrix)}\n`);
  } catch {
    console.error("::error::Invalid Pages targets. Set CLOUDFLARE_PAGES_PROJECTS to a JSON array of unique app/project entries (web, admin, site, docs; maximum 256). Single-dashboard setups may use CLOUDFLARE_PAGES_PROJECT_NAME instead.");
    process.exitCode = 1;
  }
}
