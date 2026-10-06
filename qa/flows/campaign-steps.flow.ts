import { expect, test } from "../lib/proof.ts";

test("build a campaign's steps where each one goes", async ({ page, proof }) => {
  await page.goto("/app/campaigns");
  const href = await page.getByRole("link", { name: /RevOps outreach - July/ }).first().getAttribute("href");
  await page.goto(`${href}/steps`);
  await expect(page.getByText("Contact enters the campaign")).toBeVisible();
  await proof.chapter("Campaign steps", "The sequence lays itself out from where a contact enters; waits sit on the lines");
  await proof.shot("campaign-steps", { caption: "Steps nothing leads to are flagged instead of silently never sending" });

  const picker = page.getByRole("dialog", { name: "Add a step" });
  await page.getByRole("button", { name: "Add a step here" }).first().click();
  await expect(picker).toBeVisible();
  await proof.shot("campaign-step-picker", { caption: "Emails, conditions, switches, AI steps and actions in one picker" });
  await picker.getByRole("button", { name: /^Go to a step/ }).click();
  await page.getByRole("dialog", { name: "Go to" }).getByRole("button", { name: /^Step 2 - bump/ }).click();
  const wait = page.getByRole("button", { name: /^\d+ days?$/ }).first();
  await expect(wait).toBeVisible();
  await proof.shot("connected", { caption: "Connected: the follow-up hangs under the first email with its wait on the line" });

  await wait.click();
  await expect(page.getByText(/Applies to every path into this step/)).toBeVisible();
  await proof.shot("wait-editor", { caption: "A wait is edited right on its line" });
  await page.keyboard.press("Escape");

  await page.getByRole("button", { name: "Add a step here" }).first().click();
  await expect(picker).toBeVisible();
  await picker.getByRole("textbox").pressSequentially("reply", { delay: 70 });
  await picker.getByRole("button", { name: /^If they reply/ }).click();
  await expect(page.getByRole("button", { name: "Close panel" })).toBeVisible();
  await proof.dwell();
  await proof.shot("reply-path", { caption: "A reply path becomes its own column next to Otherwise, edited in the side panel" });
  await page.getByRole("button", { name: "Close panel" }).click();

  await page.getByText("Contact enters the campaign").click();
  await expect(page.getByText("Wait before the first email")).toBeVisible();
  await proof.shot("entry-panel", { caption: "The start of the flow holds the delay before the first email" });
});
