import { expect, test } from "../lib/proof.ts";

test("build an automation by adding steps where they go", async ({ page, proof }) => {
  await page.goto("/app/automations");
  await page.getByRole("button", { name: "New automation" }).first().click();
  await expect(page).toHaveURL(/\/app\/automations\/[0-9a-f-]+$/);
  const addAtEnd = page.getByRole("button", { name: "Add a step here" });
  await expect(addAtEnd).toHaveCount(1);
  await proof.chapter("Automation builder", "Steps lay themselves out; a + adds one exactly where it goes");
  await proof.shot("empty-automation", { caption: "A new automation: the trigger and one place to add a step" });

  await addAtEnd.click();
  const picker = page.getByRole("dialog", { name: "Add a step" });
  await expect(picker).toBeVisible();
  await proof.shot("step-picker", { caption: "The step picker: searchable, grouped, keyboard driven" });
  await picker.getByRole("textbox").pressSequentially("if", { delay: 80 });
  await page.keyboard.press("Enter");

  await expect(page.getByRole("button", { name: "Add a step here" })).toHaveCount(2);
  await expect(page.getByRole("button", { name: "Close panel" })).toBeVisible();
  await proof.shot("condition-panel", { caption: "An If / else splits into Yes and No columns and opens in the side panel" });

  await page.getByRole("button", { name: "Add a step here" }).first().click();
  await expect(picker).toBeVisible();
  await picker.getByRole("textbox").pressSequentially("label", { delay: 80 });
  await picker.getByRole("button", { name: /^Add a label/ }).click();
  await expect(page.getByText("Pick a label.").first()).toBeVisible();
  await proof.shot("action-needs-setup", { caption: "A new step under Yes, flagged until its label is picked" });

  await page.getByRole("button", { name: "Close panel" }).click();
  await expect(page.getByRole("button", { name: /1 issue/ })).toBeVisible();
  await page.getByRole("button", { name: "Insert a step here" }).first().click();
  await expect(picker).toBeVisible();
  await picker.getByRole("textbox").pressSequentially("ai step", { delay: 60 });
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "Close panel" })).toBeVisible();
  await page.getByRole("button", { name: "Close panel" }).click();
  await proof.dwell();
  await proof.shot("inserted-between", { caption: "A step inserted between the trigger and the condition; everything below moves down" });

  await page.getByRole("button", { name: /issues?/ }).click();
  await proof.shot("issues-list", { caption: "Every step that still needs setup, one click away" });
  await page.keyboard.press("Escape");
});

test("route with an AI switch and send a second case to an existing step", async ({ page, proof }) => {
  await page.goto("/app/automations");
  await page.getByRole("button", { name: "New automation" }).first().click();
  await expect(page).toHaveURL(/\/app\/automations\/[0-9a-f-]+$/);
  await proof.chapter("Switches and go to", "Each case is its own column; a path can continue at a step that already exists");

  await page.getByRole("button", { name: "Add a step here" }).click();
  const picker = page.getByRole("dialog", { name: "Add a step" });
  await picker.getByRole("textbox").pressSequentially("switch", { delay: 60 });
  await page.keyboard.press("Enter");
  await expect(page.getByText("interested", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Close panel" }).click();

  await page.getByRole("button", { name: "Add a step here" }).first().click();
  await picker.getByRole("textbox").pressSequentially("create a task", { delay: 50 });
  await page.keyboard.press("Enter");
  await page.getByRole("button", { name: "Close panel" }).click();
  await proof.shot("switch-columns", { caption: "An AI switch draws one column per case" });

  await page.getByRole("button", { name: "Add a step here" }).nth(1).click();
  await picker.getByRole("button", { name: /^Go to a step/ }).click();
  await expect(page.getByRole("dialog", { name: "Go to" })).toBeVisible();
  await proof.shot("go-to-picker", { caption: "Go to lists the steps this path can continue at without looping" });
  await page.getByRole("dialog", { name: "Go to" }).getByRole("button", { name: /^Create a task/ }).click();

  const chip = page.getByRole("button", { name: /Go to Create a task/ });
  await expect(chip).toBeVisible();
  await chip.hover();
  await proof.dwell();
  await proof.shot("go-to-link", { caption: "A go-to chip instead of a crossing line; hovering it shows where it leads" });
});
