import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BoundaryFallback } from "./ErrorBoundary";

const back = vi.fn();
vi.mock("@tanstack/react-router", () => ({ useRouter: () => ({ history: { back } }) }));
afterEach(() => vi.unstubAllGlobals());

describe("route recovery", () => {
    it("reloads a document after module failure instead of resetting a cached import", () => {
        const reset = vi.fn();
        const reload = vi.fn();
        vi.stubGlobal("window", { location: { reload } });
        render(<BoundaryFallback error={new TypeError("Importing a module script failed.")} info={null} reset={reset} />);
        fireEvent.click(screen.getByRole("button", { name: "Reload" }));
        expect(reload).toHaveBeenCalledOnce();
        expect(reset).not.toHaveBeenCalled();
        expect(screen.getAllByText("Importing a module script failed.").length).toBeGreaterThan(0);
    });

    it("keeps normal retry for rendering errors", () => {
        const reset = vi.fn();
        render(<BoundaryFallback error={new Error("Rendering failed")} info={null} reset={reset} />);
        fireEvent.click(screen.getByRole("button", { name: "Retry" }));
        expect(reset).toHaveBeenCalledOnce();
        expect(screen.queryByRole("button", { name: "Reload" })).not.toBeInTheDocument();
    });
});
