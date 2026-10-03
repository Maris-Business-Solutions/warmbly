import { expect, test } from "../lib/proof.ts";

test("search contacts and open a contact's details", async ({ page, proof }) => {
  await page.goto("/app/contacts");
  await expect(page.getByRole("table")).toBeVisible();
  await proof.chapter("Contacts", "Search the list, then open one contact");

  await page.getByRole("textbox", { name: /Search by name/ }).pressSequentially("beth", { delay: 60 });
  const row = page.getByRole("row", { name: /Beth Chen/ });
  await expect(row).toBeVisible();
  await expect(page.getByRole("row", { name: /Carlos Diaz/ })).toBeHidden();
  await proof.shot("search-results", { caption: "Search narrowed to one contact" });

  await row.click();
  await expect(page.getByText("beth.chen@initech.test").last()).toBeVisible();
  await proof.dwell();
  // "Checked ... 4m ago" drifts between runs; ignored so it never reads as a change.
  await proof.shot("contact-details", { caption: "Contact details drawer", ignore: [page.getByText(/\d+[smhd] ago/)] });
});
