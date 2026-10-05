import { randomUUID } from "node:crypto";
import { appendFileSync, readFileSync } from "node:fs";

const runtimeVariables = [
  "WARMBLY_API_URL",
  "WARMBLY_APP_URL",
  "WARMBLY_TURNSTILE_KEY",
  "WARMBLY_BETA_NOTICE",
  "WARMBLY_SENTRY_DSN",
  "WARMBLY_SENTRY_ENVIRONMENT",
  "WARMBLY_POSTHOG_KEY",
  "WARMBLY_POSTHOG_HOST",
  "WARMBLY_POSTHOG_UI_HOST",
  "WARMBLY_POSTHOG_ERROR_TRACKING",
  "WARMBLY_POSTHOG_SESSION_REPLAY",
  "WARMBLY_COMPANY_LOGOS",
];
const requiredVariables = new Set([
  "WARMBLY_API_URL",
  "WARMBLY_APP_URL",
  "WARMBLY_TURNSTILE_KEY",
]);

class ConfigurationError extends Error {}

try {
  const project = JSON.parse(readFileSync(0, "utf8"));
  const branch = project.result?.production_branch;
  if (project.success !== true || typeof branch !== "string" || !branch.trim() || /[\r\n]/.test(branch)) {
    throw new ConfigurationError("Cloudflare did not return a valid Pages production branch.");
  }

  const variables = project.result.deployment_configs?.production?.env_vars ?? {};
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
