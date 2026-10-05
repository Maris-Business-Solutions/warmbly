// Inline "what do these states mean?" legend for enum badges (health states,
// job statuses). Renders a small help trigger that reveals a
// term → definition list on hover/focus, so tables stay compact but no state
// name is ever left unexplained.

import { CircleHelp } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { TONE } from "@/lib/tones";
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from "@/components/ui/tooltip";

export interface LegendEntry {
    term: string;
    description: string;
    /** Badge tone classes; use a value from TONE in lib/tones. */
    tone?: string;
}

export function StateLegend({
    label = "What do these states mean?",
    entries,
}: {
    label?: string;
    entries: LegendEntry[];
}) {
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <button
                    type="button"
                    className="inline-flex items-center gap-1 rounded text-xs text-muted-foreground transition-colors hover:text-foreground"
                >
                    <CircleHelp className="size-3.5" />
                    {label}
                </button>
            </TooltipTrigger>
            <TooltipContent
                side="bottom"
                align="start"
                className="max-w-sm p-2.5"
            >
                <dl className="space-y-1.5 py-0.5">
                    {entries.map((e) => (
                        <div key={e.term} className="flex items-start gap-2">
                            <dt className="shrink-0">
                                <Badge
                                    variant="outline"
                                    className={e.tone ?? TONE.neutral}
                                >
                                    {e.term}
                                </Badge>
                            </dt>
                            <dd className="text-xs leading-snug text-muted-foreground">
                                {e.description}
                            </dd>
                        </div>
                    ))}
                </dl>
            </TooltipContent>
        </Tooltip>
    );
}
