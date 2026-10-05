import { afterEach, describe, expect, it } from "vitest";
import type { AxiosAdapter } from "axios";
import Client from "./Client";

const original = Client.defaults.adapter;

function answer(contentType: string, data: unknown): AxiosAdapter {
    return async (config) => ({ data, status: 200, statusText: "OK", headers: { "content-type": contentType }, config });
}

describe("Client", () => {
    afterEach(() => {
        Client.defaults.adapter = original;
    });

    it("refuses a web page where the API should answer", async () => {
        Client.defaults.adapter = answer("text/html; charset=utf-8", "<!doctype html><html></html>");
        await expect(Client.get("/auth/config")).rejects.toMatchObject({ code: "api_url_not_api" });
    });

    it("matches the media type in any case", async () => {
        Client.defaults.adapter = answer("Text/HTML", "<!doctype html>");
        await expect(Client.get("/auth/config")).rejects.toMatchObject({ code: "api_url_not_api" });
    });

    it("passes JSON through", async () => {
        Client.defaults.adapter = answer("application/json; charset=utf-8", { providers: ["google"] });
        await expect(Client.get("/auth/config")).resolves.toMatchObject({ data: { providers: ["google"] } });
    });

    it("leaves a requested text or blob body alone", async () => {
        Client.defaults.adapter = answer("text/html", "<p>preview</p>");
        await expect(Client.get("/preview", { responseType: "text" })).resolves.toMatchObject({ data: "<p>preview</p>" });
    });
});
