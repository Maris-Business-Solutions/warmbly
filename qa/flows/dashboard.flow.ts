import { expect, test } from "../lib/proof.ts";

test("dashboard opens on the mailboxes page", async ({ page, proof }) => {
  await page.goto("/app/emails");
  await expect(page.getByRole("row", { name: /dev\.outbound@warmbly\.test/ })).toBeVisible();
  await proof.chapter("Mailboxes", "The signed-in dashboard landing page");
  await proof.shot("mailboxes", { caption: "Mailboxes list" });
});
