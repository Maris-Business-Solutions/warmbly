import { describe, expect, it } from "vitest";
import { companyDomainOf, companyLogosEnabled } from "./companyLogo";

describe("companyDomainOf", () => {
    it("finds the company behind a work address", () => {
        expect(companyDomainOf("dana@Acme.com")).toBe("acme.com");
        expect(companyDomainOf("sam@eu.northwind.io", "google_workspace")).toBe("eu.northwind.io");
    });

    it("leaves personal inboxes alone, by domain or by detected host", () => {
        expect(companyDomainOf("dana@gmail.com")).toBe("");
        expect(companyDomainOf("dana@hotmail.co.uk")).toBe("");
        expect(companyDomainOf("dana@someisp.net", "yahoo")).toBe("");
    });

    it("never looks up test domains or things that are not addresses", () => {
        expect(companyDomainOf("lead@bulk.test")).toBe("");
        expect(companyDomainOf("not-an-address")).toBe("");
        expect(companyDomainOf("x@localhost")).toBe("");
    });
});

describe("companyLogosEnabled", () => {
    it("is on for Warmbly Cloud and off on a self-host by default", () => {
        expect(companyLogosEnabled(false)).toBe(true);
        expect(companyLogosEnabled(true)).toBe(false);
        // Until the backend has said which it is, nothing is fetched.
        expect(companyLogosEnabled(undefined)).toBe(false);
    });
});
