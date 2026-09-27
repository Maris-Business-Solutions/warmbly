if (!process.env.VITE_SENTRY_RELEASE && process.env.CF_PAGES_COMMIT_SHA) process.env.VITE_SENTRY_RELEASE = process.env.CF_PAGES_COMMIT_SHA;import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";
import tailwindcss from "@tailwindcss/vite";
import { sentryVitePlugin } from "@sentry/vite-plugin";

// A Cloudflare Pages build sets no release, so its events are tagged with the commit it built.
if (!process.env.VITE_SENTRY_RELEASE && process.env.CF_PAGES_COMMIT_SHA) process.env.VITE_SENTRY_RELEASE = process.env.CF_PAGES_COMMIT_SHA;

// Source maps, and nothing else, is what this section decides.
//
// Uploading them is never a required build step: a fork, a self-host build or a
// local `pnpm build` configures neither backend, so nothing needs an account
// anywhere and nothing is uploaded. CI passes the credentials as build secrets
// only for the hosted release.
//
// Source maps are emitted only when something is going to upload them, so the
// shipped bundle is unchanged for everybody else. PostHog's are uploaded after
// the build by the `sourcemaps:posthog` script, which is also what deletes the
// .map files afterwards, so the Sentry plugin only deletes them when it is the
// one upload configured.
const sentryAuthToken = process.env.SENTRY_AUTH_TOKEN;
const sentryOrg = process.env.SENTRY_ORG;
const sentryProject = process.env.SENTRY_PROJECT;
const uploadToSentry = Boolean(sentryAuthToken && sentryOrg && sentryProject);
const uploadToPostHog = Boolean(process.env.POSTHOG_CLI_API_KEY && process.env.POSTHOG_CLI_PROJECT_ID);
const uploadSourceMaps = uploadToSentry || uploadToPostHog;

const sentryPlugins = uploadToSentry
    ? [
          sentryVitePlugin({
              authToken: sentryAuthToken,
              org: sentryOrg,
              project: sentryProject,
              release: { name: process.env.VITE_SENTRY_RELEASE },
              sourcemaps: { filesToDeleteAfterUpload: uploadToPostHog ? [] : ["dist/**/*.map"] },
              telemetry: false,
          }),
      ]
    : [];

export default defineConfig({
    plugins: [
        react(),
        tailwindcss(),
        ...sentryPlugins,
    ],
    build: {
        // Only when they are going to be uploaded: shipping them otherwise
        // would hand every visitor the app's original sources.
        sourcemap: uploadSourceMaps,
    },
    resolve: {
        alias: {
            "@": path.resolve(__dirname, "./src"),
        },
    },
    // Pre-bundle heavy deps so the first request to the dev server
    // doesn't trigger a cold compile of axios/framer-motion/etc. This
    // shaves ~300-500ms off the first paint locally.
    optimizeDeps: {
        include: [
            "react",
            "react-dom",
            "react-router-dom",
            "axios",
            "framer-motion",
            "@tanstack/react-query",
            "@tanstack/react-query-devtools",
            "react-hot-toast",
            "lucide-react",
            "@remixicon/react",
        ],
    },
    server: {
        // Permit Tailscale MagicDNS names (and any extra hosts via
        // VITE_ALLOWED_HOSTS) when the server is exposed with --host. Vite
        // always allows IPs + localhost; this only adds named hosts, so it's
        // inert for normal local dev. Lets `make web PUBLIC_HOST=<name>` work
        // when reached at https://<host>.<tailnet>.ts.net.
        allowedHosts: [".ts.net", ...(process.env.VITE_ALLOWED_HOSTS?.split(",").filter(Boolean) ?? [])],
        // Warm up the most-mounted entry points before the user
        // clicks them so navigation doesn't trigger a cold compile.
        warmup: {
            clientFiles: [
                "./src/main.tsx",
                "./src/app/app/layout.tsx",
                "./src/app/app/emails/page.tsx",
                "./src/app/app/campaigns/page.tsx",
                "./src/app/app/contacts/page.tsx",
            ],
        },
    },
});
