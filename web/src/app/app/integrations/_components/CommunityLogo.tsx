// A community app's uploaded logo, or a tinted initial on the same tile the
// built-in provider marks use, so both kinds of card line up.

import React from "react";

import { cn } from "@/lib/utils";

const TINTS = [
    "bg-sky-50 ring-sky-100 text-sky-700",
    "bg-indigo-50 ring-indigo-100 text-indigo-700",
    "bg-emerald-50 ring-emerald-100 text-emerald-700",
    "bg-rose-50 ring-rose-100 text-rose-700",
    "bg-amber-50 ring-amber-100 text-amber-700",
    "bg-fuchsia-50 ring-fuchsia-100 text-fuchsia-700",
];

export default function CommunityLogo({ name, url, size = 9 }: { name: string; url?: string; size?: 7 | 9 | 10 }) {
    const [broken, setBroken] = React.useState(false);
    const dim = size === 7 ? "w-7 h-7 text-[12px]" : size === 10 ? "w-10 h-10 text-[15px]" : "w-9 h-9 text-[13px]";
    if (url && !broken) {
        return (
            <img
                src={url}
                alt=""
                onError={() => setBroken(true)}
                className={cn("rounded-md ring-1 ring-slate-200 object-cover bg-white shrink-0", dim)}
            />
        );
    }
    const letter = (name.trim()[0] ?? "?").toUpperCase();
    return (
        <div
            className={cn(
                "rounded-md ring-1 inline-flex items-center justify-center font-semibold shrink-0",
                dim,
                TINTS[letter.charCodeAt(0) % TINTS.length],
            )}
        >
            {letter}
        </div>
    );
}
