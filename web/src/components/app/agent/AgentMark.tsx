// The assistant's mark. The icon is a soft blue blob with light swirling inside:
// it breathes at rest, squishes when the pointer arrives (its shine follows the
// pointer), boops on each keystroke when it `listen`s, swirls faster while a
// run works, hops in amber while a run waits on approval, and bounces in green
// when a run finishes. `variant="bare"` is a pixel spark for inline loaders.
// Motion lives in global.css (.amo).

import React from "react";
import { useReducedMotion } from "framer-motion";
import { cn } from "@/lib/utils";
import { subscribeAgentPulse } from "./agentPulse";

export type AgentMarkState = "idle" | "thinking" | "attention";

type Props = {
    size?: number;
    state?: AgentMarkState;
    // muted greys a resting mark, for one that should not compete.
    tone?: "color" | "muted";
    // Boop on composer keystrokes (pulseAgent).
    listen?: boolean;
    // Whether a run ending now succeeded; only then does the mark celebrate.
    celebrate?: boolean;
    // blob is the assistant's icon; bare is a pixel spark, for inline loaders.
    variant?: "blob" | "bare";
    className?: string;
};

export default function AgentMark({ variant = "blob", ...props }: Props) {
    return variant === "bare" ? <PixelMark {...props} /> : <BlobMark {...props} />;
}

// True for a moment after a run finishes well, so the mark can celebrate.
function useJustFinished(state: AgentMarkState, celebrate: boolean): boolean {
    const [done, setDone] = React.useState(false);
    const prev = React.useRef(state);
    React.useEffect(() => {
        if (prev.current === "thinking" && state === "idle" && celebrate) setDone(true);
        prev.current = state;
    }, [state, celebrate]);
    React.useEffect(() => {
        if (!done) return;
        const t = window.setTimeout(() => setDone(false), 1400);
        return () => window.clearTimeout(t);
    }, [done]);
    return done && state === "idle";
}

// Restarts a one-shot CSS animation class without a re-render.
function replay(el: HTMLElement | null, cls: string) {
    if (!el) return;
    el.classList.remove(cls);
    void el.offsetWidth;
    el.classList.add(cls);
}

function BlobMark({
    size = 16,
    state = "idle",
    tone = "color",
    listen = false,
    celebrate = true,
    className,
}: Omit<Props, "variant">) {
    const reduced = useReducedMotion();
    const ref = React.useRef<HTMLSpanElement>(null);
    const done = useJustFinished(state, celebrate);

    React.useEffect(() => {
        if (!listen || reduced) return;
        return subscribeAgentPulse(() => replay(ref.current, "amo-boop"));
    }, [listen, reduced]);

    // Squish on arrival and let the shine follow the pointer. Written straight
    // to the element, so moving the cursor never re-renders.
    React.useEffect(() => {
        const el = ref.current;
        if (!el || reduced) return;
        const host = (el.closest(".group") as HTMLElement | null) ?? el;
        const enter = () => replay(el, "amo-jelly");
        const move = (e: PointerEvent) => {
            const r = el.getBoundingClientRect();
            const x = (e.clientX - r.left) / r.width;
            const y = (e.clientY - r.top) / r.height;
            el.style.setProperty("--amo-sx", `${Math.min(62, Math.max(8, x * 100 - 15))}%`);
            el.style.setProperty("--amo-sy", `${Math.min(58, Math.max(8, y * 100 - 11))}%`);
        };
        const leave = () => {
            el.style.removeProperty("--amo-sx");
            el.style.removeProperty("--amo-sy");
        };
        const end = (e: AnimationEvent) => {
            if (e.animationName === "amo-jelly") el.classList.remove("amo-jelly");
            if (e.animationName === "amo-boop") el.classList.remove("amo-boop");
        };
        host.addEventListener("pointerenter", enter);
        host.addEventListener("pointermove", move);
        host.addEventListener("pointerleave", leave);
        el.addEventListener("animationend", end);
        return () => {
            host.removeEventListener("pointerenter", enter);
            host.removeEventListener("pointermove", move);
            host.removeEventListener("pointerleave", leave);
            el.removeEventListener("animationend", end);
        };
    }, [reduced]);

    const look = done ? "done" : tone === "muted" && state === "idle" ? "muted" : state;
    return (
        <span
            ref={ref}
            aria-hidden
            className={cn("amo amo-b", `amo-${look}`, className)}
            style={{ width: size, height: size, fontSize: size }}
        >
            <span className="amo-body">
                <span className="amo-swirl" />
            </span>
            <span className="amo-shine" />
        </span>
    );
}

// '#' lit, '+' half lit, '.' off. Row-major, top row first.
function pattern(rows: string[]): number[] {
    return rows
        .join("")
        .split("")
        .map((c) => (c === "#" ? 1 : c === "+" ? 0.62 : 0));
}

const SPARK = pattern([".+.", "+#+", ".+."]);
// The outer ring, clockwise from the top left.
const RING = [0, 1, 2, 5, 8, 7, 6, 3];

function PixelMark({ size = 16, state = "idle", tone = "color", celebrate = true, className }: Omit<Props, "variant">) {
    const reduced = useReducedMotion();
    const done = useJustFinished(state, celebrate);
    const cell = size * 0.26;
    const gap = Math.max(0.5, size * 0.1);
    const look = done ? "done" : tone === "muted" && state === "idle" ? "muted" : state;
    return (
        <span
            aria-hidden
            className={cn("amo amo-bare", `amo-${look}`, className)}
            style={{ width: size, height: size }}
        >
            <span className="amo-grid" style={{ gridTemplateColumns: `repeat(3, ${cell}px)`, gap }}>
                {SPARK.map((base, i) => {
                    let o = base;
                    let animation: string | undefined;
                    if (state === "thinking" && !reduced) {
                        // A light chases around the ring.
                        const r = RING.indexOf(i);
                        o = r === -1 ? 1 : 0.2;
                        if (r !== -1) animation = `amo-chase 0.88s linear ${r * 0.11}s infinite`;
                    } else if (state === "attention" && !reduced && o > 0) {
                        animation = "amo-pulse 1.2s ease-in-out infinite";
                    }
                    return (
                        <span
                            key={i}
                            className="amo-px"
                            style={{ width: cell, height: cell, animation, "--o": o } as React.CSSProperties}
                        />
                    );
                })}
            </span>
        </span>
    );
}
