// @vitest-environment node

import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { build } from "vite";
import { configuredSources, dashboardCspPlugin, inlineHashes, noncePlaceholder, origin } from "../csp";
import { dashboardCsp, mailPreviewCsp } from "../securityHeaders";

const directory = process.cwd();

describe("dashboard CSP", () => {
    it("generates hashes from Vite's final HTML asset", async () => {
        const temporary = mkdtempSync(path.join(tmpdir(), "warmbly-csp-build-"));
        try {
            writeFileSync(path.join(temporary, "index.html"), '<html><head><script>window.theme="dark";</script><style>body { margin: 0; }</style></head><body></body></html>');
            await build({ root: temporary, configFile: false, plugins: [dashboardCspPlugin({ VITE_API_URL: "https://built-api.test" })], logLevel: "silent" });
            const html = readFileSync(path.join(temporary, "dist/index.html"), "utf8");
            const template = readFileSync(path.join(temporary, "dist/csp-template.txt"), "utf8");
            for (const hash of [...inlineHashes(html, "script"), ...inlineHashes(html, "style")]) expect(template).toContain(hash);
            expect(readFileSync(path.join(temporary, "dist/csp-origins.txt"), "utf8")).toContain("https://built-api.test");
            expect(template).not.toContain(noncePlaceholder);
        } finally {
            rmSync(temporary, { recursive: true, force: true });
        }
    });

    it("hashes exact inline text, not external scripts or attributes", () => {
        const source = "\nwindow.theme = 'dark';\n";
        const hash = `'sha256-${createHash("sha256").update(source).digest("base64")}'`;
        expect(inlineHashes(`<script data-src="example">${source}</script><script src="/app.js"></script>`, "script")).toEqual([hash]);
        expect(inlineHashes(`<style>body{margin:0}</style>`, "style")).toHaveLength(1);
    });

    it("denies unlisted resource types, inline scripts and style elements", () => {
        const policy = dashboardCsp();
        expect(policy).toContain("default-src 'none'");
        expect(policy).toContain("script-src-attr 'none'");
        expect(policy).toContain("style-src 'self';");
        expect(policy).not.toContain("script-src 'unsafe-inline'");
        expect(policy).not.toContain("'unsafe-eval'");
        expect(policy).not.toContain("*");
    });

    it("keeps email markup styling in a separately script-disabled document", () => {
        expect(mailPreviewCsp).toContain("script-src 'none'");
        expect(mailPreviewCsp).toContain("frame-ancestors 'self'");
        expect(mailPreviewCsp).toContain("style-src 'unsafe-inline'");
        expect(readFileSync(path.join(directory, "public/_headers"), "utf8")).toContain(`/mail-preview.html\n  ! Content-Security-Policy\n  Content-Security-Policy: ${mailPreviewCsp}`);
        expect(readFileSync(path.join(directory, "nginx-mail-preview-headers.conf"), "utf8")).toContain(`Content-Security-Policy "${mailPreviewCsp}"`);
    });

    it("derives exact resource origins and permits separately configured realtime", () => {
        expect(configuredSources({ VITE_API_URL: "https://api.example.test/v1", VITE_POSTHOG_HOST: "https://eu.i.posthog.com", WARMBLY_CSP_CONNECT_ORIGINS: "wss://realtime.example.test/socket" })).toEqual({
            connections: ["https://api.example.test", "wss://api.example.test", "wss://realtime.example.test"],
            analytics: ["https://eu.i.posthog.com", "https://eu-assets.i.posthog.com"],
            images: ["https://api.example.test"],
        });
        expect(origin("https://public-key@sentry.example.test/123")).toBe("https://sentry.example.test");
        expect(origin("/ingest")).toBe("");
        expect(() => origin("javascript:alert(1)")).toThrow();
        expect(() => origin("https://example.test;script-src/*")).toThrow();
    });

    it("renders the same hashes and origins into Pages and nginx deployment output", () => {
        const temporary = mkdtempSync(path.join(tmpdir(), "warmbly-csp-"));
        try {
            const html = readFileSync(path.join(directory, "index.html"), "utf8");
            const scripts = inlineHashes(html, "script");
            const styles = inlineHashes(html, "style");
            writeFileSync(path.join(temporary, "csp-template.txt"), dashboardCsp({ scripts, styles, connections: ["__API_SOURCES__", "__SENTRY_SOURCE__", "__CONNECT_SOURCES__"], analytics: ["__ANALYTICS_SOURCES__"], images: ["__API_IMAGE_SOURCE__"] }));
            writeFileSync(path.join(temporary, "csp-origins.txt"), "https://built-api.test\nhttps://built-sentry.test\nhttps://eu.i.posthog.com\nwss://built-realtime.test\n");
            writeFileSync(path.join(temporary, "_headers"), readFileSync(path.join(directory, "public/_headers")));
            const env = { PATH: process.env.PATH, WARMBLY_CONFIG_OUT: path.join(temporary, "config.js"), WARMBLY_API_URL: "https://deployed-api.test", WARMBLY_CSP_CONNECT_ORIGINS: "wss://deployed-realtime.test" };
            execFileSync("sh", [path.join(directory, "docker-entrypoint.sh")], { env });
            const policy = readFileSync(path.join(temporary, "csp-policy.txt"), "utf8").trim();
            expect(policy).toContain("https://deployed-api.test wss://deployed-api.test");
            expect(policy).toContain("https://built-sentry.test");
            expect(policy).toContain("https://eu-assets.i.posthog.com");
            expect(policy).toContain("wss://deployed-realtime.test");
            expect(policy).not.toContain("built-api.test");
            expect(policy).not.toContain("__");
            for (const hash of [...scripts, ...styles]) expect(policy).toContain(hash);
            const headers = readFileSync(path.join(temporary, "_headers"), "utf8");
            expect(headers).toContain(`Content-Security-Policy: ${policy}`);
            expect(headers).toContain(`Content-Security-Policy: ${mailPreviewCsp}`);
            execFileSync("sh", [path.join(directory, "docker-entrypoint.sh")], { env });
            expect(readFileSync(path.join(temporary, "_headers"), "utf8")).toBe(headers);
            expect(() => execFileSync("sh", [path.join(directory, "docker-entrypoint.sh")], { env: { ...env, WARMBLY_API_URL: "https://example.test;script-src *" }, stdio: "pipe" })).toThrow();
        } finally {
            rmSync(temporary, { recursive: true, force: true });
        }
    });
});
