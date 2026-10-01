// ColorPicker — a real picker (saturation/value field, hue slider, hex input,
// presets) in a floating popover. Self-contained: no color library, the HSV
// math is a few lines and a dependency for it is not worth the bytes.
//
// Follows the repo's popover rules: click-away is registered in the CAPTURE
// phase so it still fires inside a dialog that stops propagation, and Escape
// closes only this layer.

import React from "react";
import { createPortal } from "react-dom";
import { AnimatePresence, motion } from "framer-motion";
import { PipetteIcon } from "lucide-react";

import { useAnchoredFloating } from "@/hooks/useAnchoredFloating";
import useClickOutside from "@/hooks/useClickOutside";

const HEX_RE = /^#[0-9a-fA-F]{6}$/;

const DEFAULT_SWATCHES = [
    "#0f172a", "#475569", "#94a3b8", "#e2e8f0", "#ffffff",
    "#0284c7", "#0891b2", "#0d9488", "#16a34a", "#65a30d",
    "#ca8a04", "#ea580c", "#dc2626", "#db2777", "#7c3aed",
];

interface HSV {
    h: number; // 0..360
    s: number; // 0..1
    v: number; // 0..1
}

function hexToRgb(hex: string): [number, number, number] {
    const v = hex.replace("#", "");
    return [parseInt(v.slice(0, 2), 16), parseInt(v.slice(2, 4), 16), parseInt(v.slice(4, 6), 16)];
}

function rgbToHex(r: number, g: number, b: number): string {
    const to = (n: number) => Math.round(Math.max(0, Math.min(255, n))).toString(16).padStart(2, "0");
    return `#${to(r)}${to(g)}${to(b)}`;
}

function hexToHsv(hex: string): HSV {
    const [r, g, b] = hexToRgb(hex).map((n) => n / 255) as [number, number, number];
    const max = Math.max(r, g, b);
    const min = Math.min(r, g, b);
    const d = max - min;
    let h = 0;
    if (d !== 0) {
        if (max === r) h = ((g - b) / d) % 6;
        else if (max === g) h = (b - r) / d + 2;
        else h = (r - g) / d + 4;
        h *= 60;
        if (h < 0) h += 360;
    }
    return { h, s: max === 0 ? 0 : d / max, v: max };
}

function hsvToHex({ h, s, v }: HSV): string {
    const c = v * s;
    const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
    const m = v - c;
    const seg = Math.floor(h / 60) % 6;
    const [r, g, b] = [
        [c, x, 0],
        [x, c, 0],
        [0, c, x],
        [0, x, c],
        [x, 0, c],
        [c, 0, x],
    ][seg];
    return rgbToHex((r + m) * 255, (g + m) * 255, (b + m) * 255);
}

// EyeDropper is Chromium-only; the button hides itself elsewhere.
interface EyeDropperCtor {
    new (): { open: () => Promise<{ sRGBHex: string }> };
}

export default function ColorPicker({
    value,
    fallback,
    onChange,
    swatches = DEFAULT_SWATCHES,
    disabled = false,
    "aria-label": ariaLabel,
}: {
    value?: string;
    /** Shown (and returned on edit) when value is empty. */
    fallback: string;
    onChange: (hex: string) => void;
    swatches?: string[];
    disabled?: boolean;
    "aria-label"?: string;
}) {
    const current = (value || fallback).toLowerCase();
    const [open, setOpen] = React.useState(false);
    const [text, setText] = React.useState(current);
    const panelRef = React.useRef<HTMLDivElement | null>(null);
    const triggerRef = React.useRef<HTMLButtonElement | null>(null);
    const { setReference, setFloating, floatingStyle } = useAnchoredFloating(open, {
        placement: "bottom-start",
        gap: 6,
    });

    React.useEffect(() => setText(current), [current]);

    useClickOutside(open, () => setOpen(false), [triggerRef, panelRef]);

    const hsv = hexToHsv(HEX_RE.test(current) ? current : fallback);

    const setRefs = React.useCallback(
        (el: HTMLDivElement | null) => {
            panelRef.current = el;
            setFloating(el);
        },
        [setFloating],
    );

    const hasEyeDropper = typeof window !== "undefined" && "EyeDropper" in window;

    async function pickFromScreen() {
        const Ctor = (window as unknown as { EyeDropper?: EyeDropperCtor }).EyeDropper;
        if (!Ctor) return;
        try {
            const res = await new Ctor().open();
            onChange(res.sRGBHex.toLowerCase());
        } catch {
            // The user dismissed the eyedropper; nothing to report.
        }
    }

    return (
        <>
            <button
                ref={(el) => {
                    triggerRef.current = el;
                    setReference(el);
                }}
                type="button"
                disabled={disabled}
                aria-label={ariaLabel ?? "Pick a color"}
                onClick={() => setOpen((o) => !o)}
                className={`size-7 rounded-md border border-slate-200 shrink-0 transition-shadow disabled:opacity-50 ${
                    open ? "ring-2 ring-sky-200" : "hover:border-slate-300"
                }`}
                style={{ backgroundColor: current }}
            />
            {typeof document !== "undefined" &&
                createPortal(
                    <AnimatePresence>
                        {open && (
                            <motion.div
                                ref={setRefs}
                                data-floating=""
                                style={floatingStyle}
                                // Opacity only: useAnchoredFloating positions
                                // with a transform, and animating scale here
                                // would overwrite it and drop the panel at 0,0.
                                initial={{ opacity: 0 }}
                                animate={{ opacity: 1 }}
                                exit={{ opacity: 0 }}
                                transition={{ duration: 0.12 }}
                                className="z-[70] w-56 rounded-lg border border-slate-200 bg-white p-2.5 shadow-[0_12px_32px_-8px_rgba(15,23,42,0.25)]"
                            >
                                <SaturationField hsv={hsv} onChange={(next) => onChange(hsvToHex(next))} />
                                <HueSlider
                                    hue={hsv.h}
                                    onChange={(h) => onChange(hsvToHex({ ...hsv, h, s: hsv.s || 1, v: hsv.v || 1 }))}
                                />

                                <div className="mt-2 flex items-center gap-1.5">
                                    <input
                                        value={text}
                                        onChange={(e) => {
                                            const v = e.target.value;
                                            setText(v);
                                            if (HEX_RE.test(v.trim())) onChange(v.trim().toLowerCase());
                                        }}
                                        onBlur={() => setText(current)}
                                        spellCheck={false}
                                        aria-label="Hex value"
                                        className={`h-7 flex-1 min-w-0 rounded-md border px-2 font-mono text-[12px] text-slate-900 outline-none transition-colors focus:border-sky-400 focus:ring-2 focus:ring-sky-100 ${
                                            HEX_RE.test(text.trim()) ? "border-slate-200" : "border-rose-300"
                                        }`}
                                    />
                                    {hasEyeDropper && (
                                        <button
                                            type="button"
                                            onClick={() => void pickFromScreen()}
                                            aria-label="Pick a color from the screen"
                                            title="Pick from screen"
                                            className="size-7 shrink-0 inline-flex items-center justify-center rounded-md border border-slate-200 text-slate-500 hover:text-slate-900 hover:bg-slate-50"
                                        >
                                            <PipetteIcon className="w-3.5 h-3.5" />
                                        </button>
                                    )}
                                </div>

                                <div className="mt-2 grid grid-cols-5 gap-1">
                                    {swatches.map((c) => (
                                        <button
                                            key={c}
                                            type="button"
                                            aria-label={`Use ${c}`}
                                            onClick={() => onChange(c)}
                                            className={`h-5 rounded border transition-shadow ${
                                                current === c.toLowerCase()
                                                    ? "border-transparent ring-2 ring-sky-400"
                                                    : "border-slate-200 hover:ring-2 hover:ring-slate-200"
                                            }`}
                                            style={{ backgroundColor: c }}
                                        />
                                    ))}
                                </div>
                            </motion.div>
                        )}
                    </AnimatePresence>,
                    document.body,
                )}
        </>
    );
}

// usePointerDrag reports normalized 0..1 coordinates for click and drag,
// including drags that leave the element.
function usePointerDrag(onMove: (x: number, y: number) => void) {
    const ref = React.useRef<HTMLDivElement | null>(null);
    const moveRef = React.useRef(onMove);
    moveRef.current = onMove;

    const emit = React.useCallback((clientX: number, clientY: number) => {
        const el = ref.current;
        if (!el) return;
        const r = el.getBoundingClientRect();
        const x = Math.min(1, Math.max(0, (clientX - r.left) / r.width));
        const y = Math.min(1, Math.max(0, (clientY - r.top) / r.height));
        moveRef.current(x, y);
    }, []);

    const onPointerDown = React.useCallback(
        (e: React.PointerEvent) => {
            e.preventDefault();
            emit(e.clientX, e.clientY);
            const move = (ev: PointerEvent) => emit(ev.clientX, ev.clientY);
            const up = () => {
                document.removeEventListener("pointermove", move);
                document.removeEventListener("pointerup", up);
            };
            document.addEventListener("pointermove", move);
            document.addEventListener("pointerup", up);
        },
        [emit],
    );

    return { ref, onPointerDown };
}

function SaturationField({ hsv, onChange }: { hsv: HSV; onChange: (next: HSV) => void }) {
    const { ref, onPointerDown } = usePointerDrag((x, y) => onChange({ ...hsv, s: x, v: 1 - y }));
    return (
        <div
            ref={ref}
            onPointerDown={onPointerDown}
            className="relative h-28 w-full rounded-md cursor-crosshair touch-none"
            style={{
                background: `linear-gradient(to top, #000, transparent), linear-gradient(to right, #fff, hsl(${hsv.h} 100% 50%))`,
            }}
        >
            <span
                className="absolute size-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-[0_0_0_1px_rgba(15,23,42,0.35)] pointer-events-none"
                style={{ left: `${hsv.s * 100}%`, top: `${(1 - hsv.v) * 100}%`, backgroundColor: hsvToHex(hsv) }}
            />
        </div>
    );
}

function HueSlider({ hue, onChange }: { hue: number; onChange: (h: number) => void }) {
    const { ref, onPointerDown } = usePointerDrag((x) => onChange(x * 360));
    return (
        <div
            ref={ref}
            onPointerDown={onPointerDown}
            className="relative mt-2 h-3 w-full rounded-full cursor-pointer touch-none"
            style={{
                background:
                    "linear-gradient(to right, #f00 0%, #ff0 17%, #0f0 33%, #0ff 50%, #00f 67%, #f0f 83%, #f00 100%)",
            }}
        >
            <span
                className="absolute top-1/2 size-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-[0_0_0_1px_rgba(15,23,42,0.35)] pointer-events-none"
                style={{ left: `${(hue / 360) * 100}%`, backgroundColor: `hsl(${hue} 100% 50%)` }}
            />
        </div>
    );
}
