import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { DashboardImage } from "./dashboard-image";

const { acquire } = vi.hoisted(() => ({ acquire: vi.fn() }));
vi.mock("@/lib/dashboardImage", () => ({ directDashboardImage: (src: string) => src.startsWith("/"), acquireDashboardImage: acquire }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe("DashboardImage", () => {
    it("renders local assets directly", () => {
        render(<DashboardImage src="/logo.svg" alt="Warmbly" />);
        expect(screen.getByAltText("Warmbly")).toHaveAttribute("src", "/logo.svg");
        expect(acquire).not.toHaveBeenCalled();
    });
    it("uses temporary display URLs without exposing the remote source", async () => {
        const release = vi.fn();
        acquire.mockReturnValue({ promise: Promise.resolve("blob:display-image"), release });
        const { unmount } = render(<DashboardImage src="https://customer.example/logo.png" alt="Logo" />);
        await waitFor(() => expect(screen.getByAltText("Logo")).toHaveAttribute("src", "blob:display-image"));
        unmount();
        expect(release).toHaveBeenCalled();
    });
    it("preserves existing failure fallback handlers", async () => {
        const failed = vi.fn();
        acquire.mockReturnValue({ promise: Promise.reject(new Error("Unavailable")), release: vi.fn() });
        render(<DashboardImage src="https://customer.example/missing.png" alt="Logo" onError={failed} />);
        await waitFor(() => expect(failed).toHaveBeenCalledTimes(1));
        expect(screen.getByAltText("Logo")).not.toHaveAttribute("src");
    });
});
