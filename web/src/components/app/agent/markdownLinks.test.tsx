import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import Markdown from "./Markdown";

describe("Markdown links", () => {
    it("routes app paths through onOpen", () => {
        const onOpen = vi.fn();
        render(<Markdown text="[Contacts](/app/contacts)" onOpen={onOpen} />);
        fireEvent.click(screen.getByRole("link", { name: "Contacts" }));
        expect(onOpen).toHaveBeenCalledWith("/app/contacts");
    });

    it.each(["//evil.example/x", "/\\evil.example/x", "javascript:void0", "data:text/html,x"])(
        "renders %s as plain text",
        (url) => {
            render(<Markdown text={`[go](${url})`} onOpen={vi.fn()} />);
            expect(screen.queryByRole("link")).not.toBeInTheDocument();
            expect(screen.getByText("go")).toBeInTheDocument();
        },
    );

    it("opens http(s) and mailto links in a new tab", () => {
        render(<Markdown text="[site](https://example.com/) [mail](mailto:a@example.com)" />);
        expect(screen.getByRole("link", { name: "site" })).toHaveAttribute("target", "_blank");
        expect(screen.getByRole("link", { name: "mail" })).toHaveAttribute("href", "mailto:a@example.com");
    });
});
