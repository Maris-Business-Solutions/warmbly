import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { clearReplyDraft, saveReplyDraft, type ReplySeed } from "./replyDraft";

export function useReplyDraft(key: string | null, draft: ReplySeed, defaults: ReplySeed) {
    const [baseline] = useState(() => JSON.stringify(defaults));
    const serialized = JSON.stringify(draft);
    // A default that settles after mount (the mailbox list loading) is not an edit.
    const hasDraft = serialized !== baseline && serialized !== JSON.stringify(defaults);
    const pending = useRef<ReplySeed | null>(null);
    const stopped = useRef(false);
    const mounted = useRef(true);
    const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
    const [saved, setSaved] = useState<string | null>(null);
    const [failed, setFailed] = useState(false);

    useLayoutEffect(() => {
        pending.current = hasDraft ? JSON.parse(serialized) : null;
    }, [serialized, hasDraft]);

    const flush = useCallback(() => {
        clearTimeout(timer.current);
        if (stopped.current) return true;
        if (!key) return !pending.current;
        return pending.current ? saveReplyDraft(key, pending.current) : clearReplyDraft(key);
    }, [key]);

    useEffect(() => {
        timer.current = setTimeout(() => {
            const ok = flush();
            setSaved(ok ? serialized : null);
            setFailed(!ok);
        }, 400);
        return () => clearTimeout(timer.current);
    }, [serialized, flush]);

    useEffect(() => {
        mounted.current = true;
        const onHidden = () => {
            if (document.visibilityState === "hidden") flush();
        };
        window.addEventListener("pagehide", flush);
        document.addEventListener("visibilitychange", onHidden);
        return () => {
            mounted.current = false;
            flush();
            window.removeEventListener("pagehide", flush);
            document.removeEventListener("visibilitychange", onHidden);
        };
    }, [flush]);

    // AnimatePresence keeps closing composers mounted; stop writes before clearing.
    const discard = useCallback(() => {
        clearTimeout(timer.current);
        const cleared = key ? clearReplyDraft(key) : true;
        stopped.current = cleared;
        setFailed(!cleared);
        return cleared;
    }, [key]);

    // A slow send must not erase edits made while it was in flight.
    const complete = useCallback((submitted: ReplySeed) => {
        clearTimeout(timer.current);
        const unchanged = JSON.stringify(pending.current) === JSON.stringify(submitted);
        stopped.current = unchanged;
        if (!unchanged) flush();
        const cleared = key ? clearReplyDraft(key, submitted) : true;
        return { close: unchanged && mounted.current, cleared };
    }, [key, flush]);

    const resume = useCallback(() => {
        stopped.current = false;
        setSaved(null);
        setFailed(false);
    }, []);

    return { hasDraft, saved: hasDraft && saved === serialized, failed, flush, discard, resume, complete };
}
