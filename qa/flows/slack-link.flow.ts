import type { Page } from "@playwright/test";
import { expect, test } from "../lib/proof.ts";

// Slack isn't configured in the local stack; only these synthetic codes are stubbed.
const preview = {
  organization_id: "11111111-1111-4111-8111-111111111111",
  organization_name: "Acme Labs",
  is_member: true,
  slack_team_id: "TQA123",
  slack_team_name: "Acme Slack",
  slack_user_id: "UQA123",
  slack_user_name: "Alex Morgan",
  slack_user_avatar: "",
  user_email: "dev@warmbly.com",
  email_matches: true,
  verify_available: true,
  expires_at: "2030-01-01T15:45:00Z",
};

async function mockPreview(page: Page, code: string, overrides: Partial<typeof preview> = {}) {
  await page.route(`**/v1/integrations/slack/link/${code}`, async (route) => {
    expect(route.request().method()).toBe("GET");
    await route.fulfill({ json: { ...preview, ...overrides } });
  });
}

async function reviewVisible(page: Page) {
  await expect(page.getByRole("heading", { name: "Connect Slack to Warmbly" })).toBeVisible();
  await expect(page.getByText("Alex Morgan", { exact: true })).toBeVisible();
  await expect(page.getByText("Acme Slack", { exact: true })).toBeVisible();
  await expect(page.getByText("dev@warmbly.com", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: "Back to dashboard" })).toBeVisible();
}

test.describe("signed out", () => {
  test.use({ signedIn: false });
  test("Slack links send signed-out users to login and preserve the destination", async ({ page, proof }) => {
    await page.goto("/slack/link?code=proof-signed-out");
    await expect(page).toHaveURL((url) =>
      url.pathname === "/auth/login" && url.searchParams.get("next") === "/slack/link?code=proof-signed-out");
    await expect(page.getByPlaceholder("name@company.com")).toBeVisible();
    await expect(page.getByRole("button", { name: "Continue", exact: true })).toBeVisible();
    await proof.chapter("Sign in first", "Real local routing preserves the Slack link as the next destination");
    await proof.dwell();
    await proof.shot("signed-out-redirect", { caption: "Real redirect to login; next retains /slack/link?code=proof-signed-out" });
  });
});

test("matching Slack email connects the account on the standalone page", async ({ page, proof }) => {
  await mockPreview(page, "proof-matching");
  const confirmations: unknown[] = [];
  await page.route("**/v1/integrations/slack/link", async (route) => {
    expect(route.request().method()).toBe("POST");
    confirmations.push(route.request().postDataJSON());
    await route.fulfill({
      status: 201,
      json: {
        id: "22222222-2222-4222-8222-222222222222",
        organization_id: preview.organization_id,
        connection_id: "33333333-3333-4333-8333-333333333333",
        slack_team_id: preview.slack_team_id,
        slack_user_id: preview.slack_user_id,
        user_id: "44444444-4444-4444-8444-444444444444",
        dm_notifications: true,
        created_at: "2030-01-01T15:30:00Z",
        updated_at: "2030-01-01T15:30:00Z",
      },
    });
  });
  await page.goto("/slack/link?code=proof-matching");
  await reviewVisible(page);
  await expect(page.getByRole("button", { name: "Connect account", exact: true })).toBeEnabled();
  await expect(page.getByText("Your Slack and Warmbly emails match.")).toBeVisible();
  await proof.chapter("Matching emails", "Fixture preview: check both identities, then connect");
  await proof.dwell();
  await proof.shot("review-matching-email", { caption: "Stubbed Slack preview: matching email permits Connect account" });
  await page.getByRole("button", { name: "Connect account", exact: true }).click();
  await expect(page.getByRole("heading", { name: "You're connected", exact: true })).toBeVisible();
  expect(confirmations).toEqual([{ code: "proof-matching" }]);
  await expect(page.getByRole("link", { name: "Return to Slack" })).toHaveAttribute("href", "https://app.slack.com/client/TQA123");
  await expect(page.getByRole("link", { name: "Slack settings", exact: true })).toHaveAttribute("href", "/app/integrations");
  await proof.chapter("Connected", "Stubbed confirmation response; no real Slack message was sent");
  await proof.dwell();
  await proof.shot("done-connected", { caption: "After Connect account: stubbed 201 shows success and both return links" });
});

test("mismatched Slack email offers verification and adapts to mobile", async ({ page, proof }) => {
  await mockPreview(page, "proof-mismatch", { email_matches: false });
  await mockPreview(page, "proof-unavailable", { email_matches: false, verify_available: false });
  await page.goto("/slack/link?code=proof-mismatch");
  await reviewVisible(page);
  const verify = page.getByRole("button", { name: "Continue with Slack", exact: true });
  await expect(verify).toBeEnabled();
  await expect(page.getByRole("button", { name: "Connect account", exact: true })).toHaveCount(0);
  await expect(page.getByText("Your Slack email is different, so Slack will confirm this account is yours. It takes one click.")).toBeVisible();
  await proof.chapter("Different emails", "Fixture preview: Continue with Slack; live OAuth not exercised");
  await proof.dwell();
  await proof.shot("review-mismatched-email", { caption: "Stubbed mismatch preview offers Continue with Slack" });
  await page.setViewportSize({ width: 375, height: 812 });
  await reviewVisible(page);
  await expect(verify).toBeInViewport();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(375);
  await proof.dwell();
  await proof.shot("review-mobile", { caption: "375×812 mobile review using stubbed mismatch preview" });
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.goto("/slack/link?code=proof-unavailable");
  await reviewVisible(page);
  await expect(page.getByRole("button", { name: "Connect account", exact: true })).toBeDisabled();
  await expect(page.getByText("Your Slack email is not the one you use for Warmbly. Sign in to Warmbly with the email on your Slack profile to link.")).toBeVisible();
  await proof.chapter("Verification unavailable", "Fixture preview: disabled action explains how to proceed");
  await proof.dwell();
  await proof.shot("review-verification-unavailable", { caption: "Stubbed unavailable verification: disabled Connect account and recovery instructions" });
});

test("Slack links explain invalid access and forward legacy URLs", async ({ page, proof }) => {
  await mockPreview(page, "proof-not-member", { is_member: false });
  await mockPreview(page, "x");
  await page.goto("/slack/link?code=proof-not-member");
  await expect(page.getByRole("heading", { name: "You're not in Acme Labs", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /Connect account|Continue with Slack/ })).toHaveCount(0);
  await proof.chapter("Workspace membership", "Fixture preview: this account is not a member");
  await proof.dwell();
  await proof.shot("not-member", { caption: "Stubbed non-member preview explains the required workspace membership" });
  const response = page.waitForResponse((res) => new URL(res.url()).pathname === "/v1/integrations/slack/link/proof-invalid");
  await page.goto("/slack/link?code=proof-invalid");
  const invalid = await response;
  expect(invalid.status()).toBe(404);
  expect(await invalid.json()).toMatchObject({ code: "slack_link_invalid" });
  await expect(page.getByRole("heading", { name: "This link has expired", exact: true })).toBeVisible();
  await proof.chapter("Expired link", "Real local API: 404 slack_link_invalid for an unknown code");
  await proof.dwell();
  await proof.shot("expired-link", { caption: "Real local 404 slack_link_invalid renders the expired-link state" });
  const previewRequests: string[] = [];
  const trackPreview = (req: { url(): string }) => {
    if (req.url().includes("/v1/integrations/slack/link/")) previewRequests.push(req.url());
  };
  page.on("request", trackPreview);
  await page.goto("/slack/link");
  await expect(page.getByRole("heading", { name: "This link is missing its code", exact: true })).toBeVisible();
  await proof.dwell();
  expect(previewRequests).toEqual([]);
  page.off("request", trackPreview);
  await proof.shot("missing-code", { caption: "Real missing-code UI; no preview request was made" });
  await page.goto("/app/slack/link?code=x");
  await expect(page).toHaveURL((url) => url.pathname === "/slack/link" && url.search === "?code=x");
  await reviewVisible(page);
  await proof.chapter("Old links still work", "Real /app/slack/link?code=x forward; stubbed review data");
  await proof.dwell();
  await proof.shot("legacy-route-forward", { caption: "Legacy route preserves ?code=x and renders the standalone page (stubbed preview)" });
});
