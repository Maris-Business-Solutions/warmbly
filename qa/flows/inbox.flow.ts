import { expect, test } from "../lib/proof.ts";

test.use({ seed: "sandbox" });

test("open a reply in the unified inbox", async ({ page, proof }) => {
  await page.goto("/app/unibox");
  const thread = page.getByRole("button", { name: /^Hana Jules .*Wayne Enterprises/ });
  await expect(thread).toBeVisible();
  // Relative times ("9h", "1d") move on their own; ignored so a re-run tomorrow is not a change.
  const times = page.getByText(/^\d+[mhdw]$/);
  await proof.chapter("Unified inbox", "Replies from every mailbox in one list");
  await proof.shot("inbox-list", { caption: "Inbox with replies across mailboxes", ignore: [times] });

  await thread.click();
  await expect(page.getByText("Thursday at 2pm works for a demo").last()).toBeVisible();
  await proof.dwell();
  await proof.shot("conversation", { caption: "The reply opened in the conversation pane", ignore: [times] });
});
