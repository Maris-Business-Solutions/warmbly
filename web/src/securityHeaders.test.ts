import { describe, expect, it } from "vitest";

import { dashboardSecurityHeaders } from "../securityHeaders";

describe("dashboardSecurityHeaders", () => {
    it("prevents framing and content sniffing in local servers", () => {
        expect(dashboardSecurityHeaders["X-Frame-Options"]).toBe("DENY");
        expect(dashboardSecurityHeaders["X-Content-Type-Options"]).toBe("nosniff");
        expect(dashboardSecurityHeaders["Content-Security-Policy"]).toContain("frame-ancestors 'none'");
    });
});
