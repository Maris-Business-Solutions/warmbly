import { randomUUID } from "node:crypto";
import { appendFileSync, readFileSync } from "node:fs";
import { apps } from "./pages-projects.mjs";

class ConfigurationError extends Error {}

try {
  const app = process.env.PAGES_APP || "web";
  if (!Object.hasOwn(apps, app)) throw new ConfigurationError("Unsupported Pages app. Use web, admin, site, or docs.");
  const project = JSON.parse(readFileSync(0, "utf8"));
  const branch = project.result?.production_branch;
  if (project.success !== true || typeof branch !== "string" || !branch.trim() || /[\r\n]/.test(branch)) {
    throw new ConfigurationError("Cloudflare did not return a valid Pages production branch.");
  }

  const variables = project.result.deployment_configs?.production?.env_vars ?? {};
  let runtimeVariables;
  let requiredVariables = new Set();
  if (app === "web" || app === "admin") {
    const entrypoint = readFileSync(new URL(`../../${app}/docker-entrypoint.sh`, import.meta.url), "utf8");
    runtimeVariables = [...new Set([...entrypoint.matchAll(/\$\{(WARMBLY_[A-Z_]+)/g)].map((match) => match[1]))]
      .filter((name) => name !== "WARMBLY_CONFIG_OUT");
    requiredVariables = new Set(["WARMBLY_API_URL", app === "web" ? "WARMBLY_APP_URL" : "WARMBLY_DASHBOARD_URL", "WARMBLY_TURNSTILE_KEY"]);
  } else {
    const prefix = app === "site" ? /^PUBLIC_[A-Z0-9_]+$/ : /^NEXT_PUBLIC_[A-Z0-9_]+$/;
    runtimeVariables = Object.keys(variables).filter((name) => prefix.test(name));
  }
  const entries = runtimeVariables.map((name) => {
    const variable = variables[name];
    if (variable != null && (variable.type !== "plain_text" || typeof variable.value !== "string")) {
      throw new ConfigurationError(`Set ${name} as a plaintext production variable in Pages; it is public browser configuration.`);
    }
    const value = variable?.value ?? "";
    if (requiredVariables.has(name) && !value.trim()) {
      throw new ConfigurationError(`Set ${name} in the Pages project's production environment before enabling release deployments.`);
    }
    let delimiter;
    do {
      delimiter = randomUUID();
    } while (value.includes(delimiter));
    return `${name}<<${delimiter}\n${value}\n${delimiter}\n`;
  });

  appendFileSync(process.env.GITHUB_ENV, entries.join(""));
  appendFileSync(process.env.GITHUB_OUTPUT, `production_branch=${branch}\n`);
} catch (error) {
  const message = error instanceof ConfigurationError
    ? error.message
    : "Failed to read Cloudflare Pages production configuration.";
  console.error(`::error::${message}`);
  process.exitCode = 1;
}
