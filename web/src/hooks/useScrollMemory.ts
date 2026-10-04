// Remembers where a scroll container was left, per list identity, and puts it
// back.
//
// Two things throw a scroll offset away and neither is under the component's
// control: unmounting (leaving the page and coming back) and `display: none`
// (the responsive panes below `md`, where the browser silently zeroes
// scrollTop). Both land the user back at the top of a list they had scrolled
// through, which is issue #396.
//
// Offsets live in a module-level map, so they survive a remount for as long as
// the tab does, and are keyed by whatever identifies the list's contents (scope
// + filters). A different key is a different list: it starts at the top.
//
// The restore is deliberately timid. It stops the moment the user touches the
// wheel, a finger or a key, and it gives up after a short window. A container
// that merely came back from `display: none` is only touched while it sits at
// the top, which is the one offset that has to be the browser's doing rather
// than the user's: a top the user scrolled to was recorded as a top. It keeps
// retrying while the list is still filling in, so a return to a list whose rows
// arrive a frame or two later still lands where it should.

import * as React from "react";

/** How long to keep trying to reach a remembered offset (ms). */
const RESTORE_WINDOW_MS = 2000;
/** Frames to wait for a short list to grow before settling for its bottom. */
const STALL_FRAMES = 30;
/** Bound on remembered lists, so a session of filter-typing can't grow it. */
const MAX_KEYS = 40;

/** Where a set of containers was left; a caller with its own churn passes its own store. */
export type ScrollStore = Map<string, number>;

const lists: ScrollStore = new Map();

function remember(positions: ScrollStore, key: string, top: number): void {
    // Re-insert so the map's iteration order is least-recently-used first.
    positions.delete(key);
    positions.set(key, top);
    while (positions.size > MAX_KEYS) {
        const oldest = positions.keys().next().value;
        if (oldest === undefined) break;
        positions.delete(oldest);
    }
}

export function useScrollMemory(
    ref: React.RefObject<HTMLElement | null>,
    key: string,
    positions: ScrollStore = lists,
): void {
    // The offset we are still trying to reach; null means "not restoring",
    // which is also what tells the scroll listener it may record again.
    const pending = React.useRef<number | null>(null);
    const deadline = React.useRef(0);
    const frame = React.useRef(0);
    const lastHeight = React.useRef(0);
    const stalled = React.useRef(0);

    const stop = React.useCallback(() => {
        pending.current = null;
        if (frame.current) cancelAnimationFrame(frame.current);
        frame.current = 0;
    }, []);

    const step = React.useCallback(() => {
        frame.current = 0;
        const el = ref.current;
        const want = pending.current;
        if (!el || want == null) return;
        if (el.clientHeight > 0) {
            const max = Math.max(el.scrollHeight - el.clientHeight, 0);
            const next = Math.min(want, max);
            if (Math.abs(el.scrollTop - next) > 1) el.scrollTop = next;
            if (next >= want - 1) return stop();
            // Short of the target: only worth holding on while rows are still
            // arriving. A list that has stopped growing is as long as it is
            // going to get, and pinning the user to its bottom for the rest of
            // the window would be the same fight this hook exists to end.
            if (el.scrollHeight === lastHeight.current) {
                if (++stalled.current > STALL_FRAMES) return stop();
            } else {
                stalled.current = 0;
                lastHeight.current = el.scrollHeight;
            }
        }
        if (performance.now() >= deadline.current) return stop();
        frame.current = requestAnimationFrame(step);
    }, [ref, stop]);

    // `fresh` means the list itself changed (new scope, new filters): it gets
    // positioned deliberately, at its own remembered offset or at the top,
    // never at wherever the previous list happened to be parked. Otherwise this
    // is a container that came back from `display: none`, where the only offset
    // worth undoing is the one the browser wrote.
    const arm = React.useCallback(
        (fresh: boolean) => {
            const el = ref.current;
            // A hidden container has nothing to position, and arming one would
            // leave a frame loop running for as long as it stays hidden. It
            // gets its turn when it comes back.
            if (!el || el.clientHeight === 0) return;
            const want = positions.get(key) ?? 0;
            if (want <= 0) {
                if (fresh) el.scrollTop = 0;
                return;
            }
            if (!fresh && el.scrollTop !== 0) return;
            pending.current = want;
            deadline.current = performance.now() + RESTORE_WINDOW_MS;
            lastHeight.current = el.scrollHeight;
            stalled.current = 0;
            if (!frame.current) frame.current = requestAnimationFrame(step);
        },
        [ref, key, step, positions],
    );

    React.useLayoutEffect(() => {
        const el = ref.current;
        if (!el) return;

        const onScroll = () => {
            // A hidden container reports 0 for everything; recording that would
            // overwrite the very offset we are holding on to.
            if (pending.current != null || el.clientHeight === 0) return;
            remember(positions, key, el.scrollTop);
        };
        // Any deliberate move hands control back to the user for good.
        const onUserScroll = () => stop();

        el.addEventListener("scroll", onScroll, { passive: true });
        el.addEventListener("wheel", onUserScroll, { passive: true });
        el.addEventListener("touchmove", onUserScroll, { passive: true });
        el.addEventListener("keydown", onUserScroll);
        // A scrollbar drag produces neither a wheel nor a touch, only this.
        el.addEventListener("pointerdown", onUserScroll);

        // Catches a pane coming back from `display: none` without this
        // component re-rendering.
        let wasVisible = el.clientHeight > 0;
        const observer = new ResizeObserver(() => {
            const visible = el.clientHeight > 0;
            if (visible && !wasVisible) arm(false);
            wasVisible = visible;
        });
        observer.observe(el);

        arm(true);

        return () => {
            stop();
            observer.disconnect();
            el.removeEventListener("scroll", onScroll);
            el.removeEventListener("wheel", onUserScroll);
            el.removeEventListener("touchmove", onUserScroll);
            el.removeEventListener("keydown", onUserScroll);
            el.removeEventListener("pointerdown", onUserScroll);
        };
    }, [ref, key, arm, stop, positions]);

    // ResizeObserver is allowed to skip an element with no box, so a pane that
    // is hidden and shown again may never report the round trip. Every render
    // is the other moment that can happen, and the check costs nothing unless
    // the container is sitting at a top it did not choose.
    React.useLayoutEffect(() => {
        arm(false);
    });
}
