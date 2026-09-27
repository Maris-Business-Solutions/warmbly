// React Query's structural sharing, extended to Dates: the default never reuses one, so every
// refetch gave unchanged data a new identity and reset the effects keyed on it.
export default function shareDeep(prev: unknown, next: unknown): unknown {
    if (prev === next) return prev;

    if (prev instanceof Date && next instanceof Date) {
        return Object.is(prev.getTime(), next.getTime()) ? prev : next;
    }

    const arrays = Array.isArray(prev) && Array.isArray(next);
    if (!arrays && !(isPlain(prev) && isPlain(next))) return next;

    const before = prev as Record<string, unknown>;
    const after = next as Record<string, unknown>;
    const keys = Object.keys(after);
    const out: Record<string, unknown> = arrays ? ([] as unknown as Record<string, unknown>) : {};
    let same = Object.keys(before).length === keys.length;

    for (const key of keys) {
        const value = shareDeep(before[key], after[key]);
        out[key] = value;
        if (value !== before[key] || !(key in before)) same = false;
    }

    return same ? prev : out;
}

function isPlain(value: unknown): value is Record<string, unknown> {
    if (value === null || typeof value !== "object") return false;
    const prototype = Object.getPrototypeOf(value);
    return prototype === Object.prototype || prototype === null;
}
