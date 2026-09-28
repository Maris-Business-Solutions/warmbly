// Issue #207: a 1,000-row import failed every row with "invalid custom field
// key: Company Mobile". The client now enforces the same rule the API does, so
// a name it cannot use is caught on the mapping screen. These cases mirror
// internal/utils/json.go — if that rule moves, this is where it shows up.

import { describe, it, expect } from "vitest";
import {
    customKeyOf,
    customKeyStatus,
    derivePreview,
    foldCustomKey,
    isValidCustomKey,
    mappingProblem,
    matchExistingKey,
    normalizeCustomKey,
    suggestCustomKey,
    targetIdentity,
} from "./importShared";
import type { ImportColumnMapping, ImportPreview } from "@/lib/api/client/app/contacts/importContacts";

describe("custom field names", () => {
    it("accepts what the template engine can resolve", () => {
        for (const key of ["role", "Job_Title", "Company Mobile", "first-name", "plan tier 2"]) {
            expect(isValidCustomKey(key)).toBe(true);
        }
    });

    it("rejects names nothing could merge into an email", () => {
        for (const key of ["", "   ", "Company/Mobile", "Revenue ($)", "a.b", "тест"]) {
            expect(isValidCustomKey(key)).toBe(false);
        }
    });

    it("collapses stray whitespace so one column is one field", () => {
        expect(normalizeCustomKey("  Company   Mobile ")).toBe("Company Mobile");
        expect(isValidCustomKey("  Company   Mobile ")).toBe(true);
    });

    it("turns a spreadsheet header into a usable name", () => {
        expect(suggestCustomKey("Company Mobile")).toBe("Company Mobile");
        expect(suggestCustomKey("Revenue ($)")).toBe("Revenue");
        expect(suggestCustomKey("Annual Revenue (USD)")).toBe("Annual Revenue USD");
        expect(suggestCustomKey("###")).toBe("");
    });
});

describe("mappingProblem", () => {
    const email: ImportColumnMapping = { index: 0, target: "email" };

    it("passes a mapping the API would accept", () => {
        expect(
            mappingProblem([email, { index: 5, target: "custom", custom_key: "Company Mobile" }]),
        ).toBeNull();
    });

    it("names the column when a custom field has no name", () => {
        expect(mappingProblem([email, { index: 5, target: "custom", custom_key: "  " }])).toBe(
            "Column 6 needs a custom field name.",
        );
    });

    it("explains an unusable name instead of letting the import fail row by row", () => {
        const msg = mappingProblem([email, { index: 5, target: "custom", custom_key: "Company/Mobile" }]);
        expect(msg).toContain("Company/Mobile");
        expect(msg).toContain("letters");
    });

    it("still requires an email column", () => {
        expect(mappingProblem([{ index: 1, target: "first_name" }])).toBe("Map a column to Email.");
    });

    it("understands the legacy custom:<key> spelling", () => {
        expect(mappingProblem([email, { index: 2, target: "custom:plan_tier" }])).toBeNull();
        expect(mappingProblem([email, { index: 2, target: "custom:plan/tier" }])).toContain("plan/tier");
    });
});

describe("existing custom fields", () => {
    const existing = ["Industry", "company_url", "industry", "Total Score"];

    it("finds the field a header names, ignoring case and separators", () => {
        expect(matchExistingKey("Industry", existing)).toBe("Industry");
        expect(matchExistingKey("INDUSTRY", existing)).toBe("Industry");
        expect(matchExistingKey("industry", existing)).toBe("industry");
        expect(matchExistingKey("Company URL", existing)).toBe("company_url");
        expect(matchExistingKey("total-score", existing)).toBe("Total Score");
        expect(matchExistingKey("Website", existing)).toBeUndefined();
        expect(matchExistingKey("###", existing)).toBeUndefined();
    });

    it("tells an existing field from a near-duplicate and a new one", () => {
        expect(customKeyStatus("company_url", existing)).toEqual({ kind: "existing" });
        expect(customKeyStatus("Company-URL", existing)).toEqual({ kind: "similar", existing: "company_url" });
        expect(customKeyStatus("Website", existing)).toEqual({ kind: "new" });
    });

    it("folds the way the server does", () => {
        expect(foldCustomKey(" Company.URL ")).toBe("companyurl");
        expect(foldCustomKey("Revenue ($)")).toBe("revenue");
        expect(foldCustomKey("Area m²")).toBe("aream²");
    });

    it("reads both spellings of a custom mapping", () => {
        expect(customKeyOf({ index: 1, target: "custom", custom_key: " Company  Mobile " })).toBe("Company Mobile");
        expect(customKeyOf({ index: 1, target: "custom:Role" })).toBe("Role");
        expect(customKeyOf({ index: 1, target: "custom:Role", custom_key: "" })).toBe("Role");
        expect(customKeyOf({ index: 1, target: "email" })).toBe("");
    });

    it("names where a mapping writes, except targets that take many columns", () => {
        expect(targetIdentity({ index: 1, target: "custom", custom_key: "Industry" })).toBe("custom:Industry");
        expect(targetIdentity({ index: 1, target: "phone" })).toBe("phone");
        expect(targetIdentity({ index: 1, target: "categories" })).toBeNull();
        expect(targetIdentity({ index: 1, target: "ignore" })).toBeNull();
        expect(targetIdentity({ index: 1, target: "custom", custom_key: "" })).toBeNull();
    });
});

const withHeader: ImportPreview = {
    filename: "leads.csv",
    format: "csv",
    total_rows: 2,
    columns: ["Email", "Column 2"],
    has_header: true,
    sample_rows: [
        ["a@x.test", "Ada"],
        ["b@x.test", "Bo"],
    ],
    suggested_mapping: [],
};

describe("derivePreview", () => {
    it("returns the preview untouched for the detected choice", () => {
        expect(derivePreview(withHeader, true)).toBe(withHeader);
    });

    it("turns the header row into data when the file has none", () => {
        const got = derivePreview(withHeader, false);
        expect(got.columns).toEqual(["Column 1", "Column 2"]);
        // A header cell that was blank is blank again as data.
        expect(got.sample_rows[0]).toEqual(["Email", ""]);
        expect(got.total_rows).toBe(3);
    });

    it("promotes the first data row to the header", () => {
        const noHeader: ImportPreview = { ...withHeader, columns: ["Column 1", "Column 2"], has_header: false };
        const got = derivePreview(noHeader, true);
        expect(got.columns).toEqual(["a@x.test", "Ada"]);
        expect(got.sample_rows).toEqual([["b@x.test", "Bo"]]);
        expect(got.total_rows).toBe(1);
    });
});
