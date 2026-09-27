// A whole RFC3339 timestamp, the shape Go writes a time.Time as. Anchored at both
// ends so free text that merely starts with a date (a subject, a snippet) stays text.
const RFC3339 = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/

// User-authored string maps: a value that looks like a timestamp is still the text someone typed.
const VERBATIM_KEYS = new Set(["custom_fields"])

export default function reviveDates<T>(obj: T): T {
    if (obj === null || obj === undefined) return obj

    if (typeof obj === "string" && RFC3339.test(obj)) {
        return new Date(obj) as unknown as T
    }

    if (Array.isArray(obj)) {
        return obj.map((v) => reviveDates(v)) as unknown as T
    }

    if (typeof obj === "object") {
        // Blob, Date, File, and other class instances are already fully decoded.
        const prototype = Object.getPrototypeOf(obj)
        if (prototype !== Object.prototype && prototype !== null) return obj

        const entries = Object.entries(obj as Record<string, unknown>).map(
            ([key, value]) => [key, VERBATIM_KEYS.has(key) ? value : reviveDates(value)]
        )
        return Object.fromEntries(entries) as T
    }

    return obj
}
