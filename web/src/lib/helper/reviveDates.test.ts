import { describe, expect, it } from "vitest";
import reviveDates from "./reviveDates";

describe("reviveDates", () => {
    it("revives the timestamps Go writes, with and without fractions or an offset", () => {
        const r = reviveDates({
            a: "2026-09-27T11:53:26Z",
            b: "2026-09-27T11:53:26.123456789Z",
            c: "2026-09-27T13:53:26+02:00",
        }) as unknown as Record<string, Date>;
        expect(r.a).toBeInstanceOf(Date);
        expect(r.b.toISOString()).toBe("2026-09-27T11:53:26.123Z");
        expect(r.c.toISOString()).toBe("2026-09-27T11:53:26.000Z");
    });

    it("leaves text that only starts like a timestamp as text", () => {
        const subject = "2026-09-27T03:00Z nightly backup failed";
        const r = reviveDates({ subject, snippet: "2026-09-27T03:00:00Z: disk full", local: "2026-09-27T10:00" });
        expect(r).toEqual({ subject, snippet: "2026-09-27T03:00:00Z: disk full", local: "2026-09-27T10:00" });
    });

    it("keeps a day string and a custom field value as the string they are", () => {
        const r = reviveDates({ date: "2026-09-27", custom_fields: { renewal: "2026-09-27T00:00:00Z" } });
        expect(r).toEqual({ date: "2026-09-27", custom_fields: { renewal: "2026-09-27T00:00:00Z" } });
    });

    it("revives inside arrays and nested objects", () => {
        const r = reviveDates({ data: [{ created_at: "2026-09-27T00:00:00Z" }] }) as unknown as { data: { created_at: Date }[] };
        expect(r.data[0].created_at).toBeInstanceOf(Date);
    });
});
