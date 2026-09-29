// The scope rail's sections fold and their rows can be hidden.
//
// Both are per-browser preferences that live in the persisted store, so what is
// pinned here is the part that is easy to get wrong: the fold survives, a
// folded section still shows the scope you are on, hiding a row only takes it
// off the rail (the active scope never disappears), and a stored value that is
// not what the setters would have written cannot reach the screen.

import React from "react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { useAppStore } from "@/stores";
import { sanitizeUniboxRailHidden } from "@/stores/slices/uiSlice";
import { ScopeRail, type UniboxScope } from "./ScopeRail";

const overview = vi.hoisted(() => ({
  data: {
    total: 12,
    unread: 3,
    today: 0,
    week: 0,
    snoozed: 0,
    awaiting_reply: 0,
    automated: 0,
    automated_unread: 0,
    awaiting_agent_draft: 0,
    scheduled_pending: 0,
    scheduled_pending_max: 100,
    folders: [
      { folder: "inbox", unread: 3, total: 9 },
      { folder: "spam", unread: 0, total: 1 },
    ],
    mailboxes: [{ id: "m1", email: "me@example.com", name: "Me", unread: 2, total: 5 }],
    tags: [],
    categories: [{ id: "c1", title: "Interested", color: "#0ea5e9", unread: 1, total: 4 }],
  },
}));

vi.mock("@/lib/api/hooks/app/unibox/useUniboxOverview", () => ({
  default: () => ({ data: overview.data, isPending: false }),
}));
vi.mock("@/lib/api/hooks/app/unibox/useMarkSeen", () => ({
  default: () => ({ mutate: () => {} }),
}));
vi.mock("@/components/app/unibox/compose/ComposeDraftsItem", () => ({
  default: () => null,
}));
// Exit animations never finish in jsdom; a closed menu unmounts at once instead.
vi.mock("framer-motion", async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  AnimatePresence: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
// The count-up tween needs a real animation frame; a plain number is enough here.
vi.mock("@/components/ui/AnimatedNumber", () => ({
  default: ({ value }: { value: number }) => <>{value}</>,
}));

// The toggle's name grows by the dot's screen-reader text when a highlighted
// count is folded away, so match on how it starts.
const MAIL_TOGGLE = /^Mail(?!boxes| section)/;
const DOT = /highlighted count folded away/;

// The header's always-visible pencil.
function startEditing(section: "Mail" | "Views") {
  fireEvent.click(screen.getByRole("button", { name: `Edit ${section} rows` }));
}

function mountRail(scope: UniboxScope = { kind: "all" }) {
  return render(<ScopeRail scope={scope} onChange={() => {}} />);
}

beforeEach(() => {
  useAppStore.setState({ uniboxRailFolded: {}, uniboxRailHidden: [] });
});

afterEach(() => {
  cleanup();
});

describe("ScopeRail sections", () => {
  it("starts expanded with nothing hidden and a Mail header over the mail rows", () => {
    mountRail();
    expect(screen.getByRole("button", { name: "Mail", expanded: true })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Views", expanded: true })).toBeTruthy();
    for (const label of ["All mail", "Inbox", "Spam", "Trash", "Scheduled"]) {
      expect(screen.getByText(label)).toBeTruthy();
    }
  });

  it("folds a section, hides its rows and remembers it in the persisted store", () => {
    mountRail();
    fireEvent.click(screen.getByRole("button", { name: MAIL_TOGGLE }));

    expect(useAppStore.getState().uniboxRailFolded.mail).toBe(true);
    expect(screen.getByRole("button", { name: MAIL_TOGGLE, expanded: false })).toBeTruthy();
    expect(screen.queryByText("Spam")).toBeNull();
    expect(screen.queryByText("Inbox")).toBeNull();

    // Part of what the store writes to storage, not just in-memory state.
    const persisted = useAppStore.persist.getOptions().partialize?.(useAppStore.getState()) as
      | { uniboxRailFolded?: Record<string, boolean> }
      | undefined;
    expect(persisted?.uniboxRailFolded).toEqual({ mail: true });

    fireEvent.click(screen.getByRole("button", { name: MAIL_TOGGLE }));
    expect(screen.getByText("Spam")).toBeTruthy();
  });

  it("keeps the row you are on when the section is folded", () => {
    useAppStore.setState({ uniboxRailFolded: { mail: true } });
    mountRail({ kind: "folder", folder: "spam" });
    expect(screen.getByText("Spam")).toBeTruthy();
    expect(screen.queryByText("Inbox")).toBeNull();
  });

  it("keeps the active mailbox visible in a folded Mailboxes section", () => {
    useAppStore.setState({ uniboxRailFolded: { mailboxes: true } });
    mountRail({ kind: "mailbox", mailboxId: "m1" });
    expect(screen.getByText("me@example.com")).toBeTruthy();

    cleanup();
    mountRail();
    expect(screen.queryByText("me@example.com")).toBeNull();
  });

  it("flags a highlighted count folded out of sight with a dot, and drops it when the section opens", () => {
    useAppStore.setState({ uniboxRailFolded: { mail: true } });
    mountRail({ kind: "folder", folder: "spam" });
    expect(screen.getByRole("button", { name: "Mail, highlighted count folded away" })).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: MAIL_TOGGLE }));
    expect(screen.queryByText(DOT)).toBeNull();
  });

  it("says highlighted count, not unread, because Scheduled raises the dot too", () => {
    const saved = overview.data;
    overview.data = {
      ...saved,
      unread: 0,
      scheduled_pending: 5,
      folders: [
        { folder: "inbox", unread: 0, total: 9 },
        { folder: "spam", unread: 0, total: 1 },
      ],
    };
    try {
      useAppStore.setState({ uniboxRailFolded: { mail: true } });
      mountRail({ kind: "folder", folder: "spam" });
      expect(screen.getByText(DOT)).toBeTruthy();
      expect(screen.queryByText(/unread/i)).toBeNull();
    } finally {
      overview.data = saved;
    }
  });

  it("does not raise the dot for rows the user hid", () => {
    useAppStore.setState({
      uniboxRailFolded: { mail: true },
      uniboxRailHidden: ["folder:inbox", "unread"],
    });
    mountRail({ kind: "folder", folder: "spam" });
    expect(screen.queryByText(DOT)).toBeNull();
  });

  it("marks the row you are on with aria-current", () => {
    mountRail({ kind: "unread" });
    const current = document.querySelectorAll("[aria-current]");
    expect(current).toHaveLength(1);
    expect(current[0].textContent).toContain("Unread");
  });

  it("folds Views on its own", () => {
    mountRail();
    expect(screen.getByText("Hot leads")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Views" }));
    expect(useAppStore.getState().uniboxRailFolded.views).toBe(true);
    expect(screen.queryByText("Hot leads")).toBeNull();
    // Another section is untouched.
    expect(screen.getByText("Inbox")).toBeTruthy();
  });
});

describe("ScopeRail edit mode", () => {
  it("hides a row through Edit, uncheck, Done", () => {
    mountRail();
    startEditing("Mail");

    // Every row is a checkbox, all checked to start with.
    const spam = screen.getByRole("checkbox", { name: "Spam" });
    expect(spam.getAttribute("aria-checked")).toBe("true");
    fireEvent.click(spam);
    expect(screen.getByRole("checkbox", { name: "Spam" }).getAttribute("aria-checked")).toBe("false");
    // Still listed while editing, so it can be turned back on.
    expect(screen.getByText("Spam")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Done editing Mail" }));
    expect(useAppStore.getState().uniboxRailHidden).toEqual(["folder:spam"]);
    expect(screen.queryByText("Spam")).toBeNull();
    expect(screen.getByText("Trash")).toBeTruthy();
  });

  it("the pencil does not fold the section", () => {
    mountRail();
    fireEvent.click(screen.getByRole("button", { name: "Edit Mail rows" }));
    expect(useAppStore.getState().uniboxRailFolded.mail).toBeUndefined();
    expect(screen.getByText("Spam")).toBeTruthy();
    // One click enters edit mode: no menu in between.
    expect(screen.queryByRole("menuitem")).toBeNull();
    expect(screen.getByRole("button", { name: "Done editing Mail" })).toBeTruthy();
  });

  it("counts hidden rows beside the pencil, open or folded, and drops the count when they are shown", () => {
    mountRail();
    expect(screen.queryByText(/\d+ hidden/)).toBeNull();

    startEditing("Mail");
    fireEvent.click(screen.getByRole("checkbox", { name: "Spam" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Trash" }));
    // Not shown while editing: every row is on screen then.
    expect(screen.queryByText("2 hidden")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Done editing Mail" }));
    expect(screen.getByText("2 hidden")).toBeTruthy();
    // Views has none of its own.
    expect(screen.queryByText("1 hidden")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: MAIL_TOGGLE }));
    expect(screen.getByText("2 hidden")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: MAIL_TOGGLE }));

    startEditing("Mail");
    fireEvent.click(screen.getByRole("checkbox", { name: "Spam" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Trash" }));
    fireEvent.click(screen.getByRole("button", { name: "Done editing Mail" }));
    expect(screen.queryByText(/\d+ hidden/)).toBeNull();
  });

  it("does not navigate while editing and drops the folder menu", () => {
    const onChange = vi.fn();
    render(<ScopeRail scope={{ kind: "all" }} onChange={onChange} />);
    expect(screen.getByLabelText("Inbox folder actions")).toBeTruthy();

    startEditing("Mail");
    fireEvent.click(screen.getByRole("checkbox", { name: "Inbox" }));
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.queryByLabelText("Inbox folder actions")).toBeNull();
  });

  it("shows hidden rows and unfolds the section while editing", () => {
    useAppStore.setState({ uniboxRailFolded: { mail: true }, uniboxRailHidden: ["folder:trash"] });
    mountRail();
    expect(screen.queryByText("Trash")).toBeNull();

    startEditing("Mail");
    expect(screen.getByRole("checkbox", { name: "Trash" }).getAttribute("aria-checked")).toBe("false");
    expect(screen.getByRole("checkbox", { name: "Inbox" })).toBeTruthy();
  });

  it("ends edit mode on Escape", () => {
    mountRail();
    startEditing("Mail");
    expect(screen.getAllByRole("checkbox").length).toBeGreaterThan(0);

    fireEvent.keyDown(screen.getByRole("checkbox", { name: "Inbox" }), { key: "Escape" });
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
    expect(screen.getByRole("button", { name: "Edit Mail rows" })).toBeTruthy();
  });

  it("holds the fold while editing, so Done never folds by surprise", () => {
    mountRail();
    startEditing("Mail");
    const toggle = screen.getByRole("button", { name: MAIL_TOGGLE });
    expect((toggle as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(toggle);
    expect(useAppStore.getState().uniboxRailFolded.mail).toBeUndefined();

    fireEvent.click(screen.getByRole("button", { name: "Done editing Mail" }));
    expect(screen.getByText("Spam")).toBeTruthy();
  });

  it("moves focus into the checkboxes and back to the options button", () => {
    mountRail();
    startEditing("Mail");
    expect(document.activeElement).toBe(screen.getByRole("checkbox", { name: "All mail" }));

    fireEvent.keyDown(document.activeElement as Element, { key: "Escape" });
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Edit Mail rows" }));

    startEditing("Mail");
    fireEvent.click(screen.getByRole("button", { name: "Done editing Mail" }));
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Edit Mail rows" }));
  });

  it("edits Views separately from Mail", () => {
    mountRail();
    startEditing("Views");
    fireEvent.click(screen.getByRole("checkbox", { name: "Hot leads" }));
    fireEvent.click(screen.getByRole("button", { name: "Done editing Views" }));
    expect(useAppStore.getState().uniboxRailHidden).toEqual(["view:hot"]);
    expect(screen.queryByText("Hot leads")).toBeNull();
    expect(screen.getByText("Follow up")).toBeTruthy();
  });

  it("keeps the header, and so Edit, when every row is hidden", () => {
    useAppStore.setState({
      uniboxRailHidden: [
        "all", "folder:inbox", "unread", "awaiting", "agent_drafts", "snoozed",
        "folder:drafts", "folder:sent", "scheduled", "folder:archive", "folder:spam", "folder:trash",
      ],
    });
    mountRail({ kind: "mailbox", mailboxId: "m1" });
    expect(screen.queryByText("Inbox")).toBeNull();
    expect(screen.getByRole("button", { name: "Edit Mail rows" })).toBeTruthy();
  });
});

describe("hidden rows", () => {
  it("never hide the active scope", () => {
    useAppStore.setState({ uniboxRailHidden: ["folder:spam"] });
    mountRail({ kind: "folder", folder: "spam" });
    expect(screen.getByText("Spam")).toBeTruthy();

    cleanup();
    mountRail({ kind: "all" });
    expect(screen.queryByText("Spam")).toBeNull();
  });
});

describe("sanitizeUniboxRailHidden", () => {
  it("drops anything that is not a string and removes duplicates", () => {
    expect(sanitizeUniboxRailHidden(["folder:spam", 4, null, {}, "folder:spam", "view:hot"])).toEqual([
      "folder:spam",
      "view:hot",
    ]);
    expect(sanitizeUniboxRailHidden("folder:spam")).toEqual([]);
    expect(sanitizeUniboxRailHidden(undefined)).toEqual([]);
  });

  it("runs on rehydration, where the setters are bypassed", async () => {
    const original = useAppStore.persist.getOptions().storage;
    useAppStore.persist.setOptions({
      storage: {
        getItem: () =>
          ({
            state: {
              uniboxRailHidden: ["folder:spam", 7, "folder:spam"],
              uniboxRailFolded: { mail: true, views: "yes" },
            },
          }) as never,
        setItem: () => {},
        removeItem: () => {},
      },
    });
    try {
      await useAppStore.persist.rehydrate();
      expect(useAppStore.getState().uniboxRailHidden).toEqual(["folder:spam"]);
      expect(useAppStore.getState().uniboxRailFolded).toEqual({ mail: true });
    } finally {
      useAppStore.persist.setOptions({ storage: original });
    }
  });
});
