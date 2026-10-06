import { DashboardImage } from "@/components/ui/dashboard-image";

// A community app's uploaded logo, or a tinted initial on the same tile the
// built-in provider marks use, so both kinds of card line up.

import React from "react";

import { cn } from "@/lib/utils";

import { GLYPH_DIMS, type GlyphSize } from "./glyphSize";

const TINTS = [
    "bg-sky-50 ring-sky-100 text-sky-700",
    "bg-indigo-50 ring-indigo-100 text-indigo-700",
    "bg-emerald-50 ring-emerald-100 text-emerald-700",
    "bg-rose-50 ring-rose-100 text-rose-700",
    "bg-amber-50 ring-amber-100 text-amber-700",
    "bg-fuchsia-50 ring-fuchsia-100 text-fuchsia-700",
];

export default function CommunityLogo({ name, url, size = 9 }: { name: string; url?: string; size?: GlyphSize }) {
    const [brokenUrl, setBrokenUrl] = React.useState<string | null>(null);
    const broken = brokenUrl === url;
    const dim = `${GLYPH_DIMS[size].tile} ${GLYPH_DIMS[size].text}`;
    if (url && !broken) {
        return (
            <DashboardImage
                src={url}
                alt=""
                onError={() => setBrokenUrl(url)}
                className={cn("ring-1 ring-slate-200 object-cover bg-white shrink-0", dim)}
            />
        );
    }
    const letter = (name.trim()[0] ?? "?").toUpperCase();
    return (
        <div
            className={cn(
                "ring-1 inline-flex items-center justify-center font-semibold shrink-0",
                dim,
                TINTS[letter.charCodeAt(0) % TINTS.length],
            )}
        >
            {letter}
        </div>
    );
}
