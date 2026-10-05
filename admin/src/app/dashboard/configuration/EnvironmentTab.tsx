// Configuration, environment: the resolved environment of the running
// backend, read only. Nothing here can be written from the API, so the tab's
// whole job is to answer "is my variable actually being picked up, and does
// it need a restart to change?". Sensitive keys show a fingerprint, never a
// value.

import { useMemo, useState } from "react";
import { ExternalLink, Lock, Search, Settings2, Terminal } from "lucide-react";
import { ErrorState } from "@/components/ErrorState";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState, StatusBadge } from "@/components/ui/kit";
import type { Tone } from "@/lib/tones";
import { SettingsGroup } from "./SettingsLayout";
import { docsUrl } from "@/lib/docs";
import { useQuery } from "@tanstack/react-query";
import {
    getInstanceConfig,
    type ConfigSource,
    type InstanceConfigEntry,
    type RuntimeChangeable,
} from "@/lib/api/client/admin/instance";

const GROUP_LABELS: Record<string, string> = {
    deployment: "Deployment",
    addresses: "Addresses",
    database: "Database",
    cache: "Cache",
    mail: "Platform mail",
    auth: "Authentication",
    encryption: "Encryption",
    storage: "Storage",
    eventbus: "Event bus",
    workers: "Workers",
    tracking: "Tracking",
    captcha: "Captcha",
    observability: "Observability",
};

const GROUP_ORDER = Object.keys(GROUP_LABELS);

const SOURCE_STYLES: Record<ConfigSource, { label: string; tone: Tone; title: string }> = {
    env: {
        label: "env",
        tone: "success",
        title: "Read from this process's environment.",
    },
    default: {
        label: "default",
        tone: "neutral",
        title: "No environment variable set, so the built-in default applies.",
    },
    derived: {
        label: "derived",
        tone: "info",
        title: "Computed from other values rather than set directly.",
    },
    unset: {
        label: "unset",
        tone: "warning",
        title: "Not set and there is no default: the feature it controls is off.",
    },
};

const RESTART_STYLES: Record<RuntimeChangeable, { label: string; tone: Tone }> = {
    "boot-only": {
        label: "Restart to change",
        tone: "neutral",
    },
    "per-request": {
        label: "Takes effect immediately",
        tone: "success",
    },
};

// "eventbus" -> "Eventbus". Keeps a group the frontend has not learned yet
// rendering instead of disappearing.
function groupLabel(group: string): string {
    if (GROUP_LABELS[group]) return GROUP_LABELS[group];
    const spaced = group.replace(/[-_]+/g, " ").trim();
    if (!spaced) return "Other";
    return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

function groupRank(group: string): number {
    const i = GROUP_ORDER.indexOf(group);
    return i === -1 ? GROUP_ORDER.length : i;
}

function matches(entry: InstanceConfigEntry, needle: string): boolean {
    if (!needle) return true;
    const q = needle.toLowerCase();
    if (entry.key.toLowerCase().includes(q)) return true;
    if (groupLabel(entry.group).toLowerCase().includes(q)) return true;
    if (entry.effect.toLowerCase().includes(q)) return true;
    // A sensitive entry never carries its value, so there is nothing to match.
    if (!entry.sensitive && entry.value.toLowerCase().includes(q)) return true;
    return false;
}

interface EnvironmentTabProps {
    onSwitchTab?: (tab: "settings") => void;
}

export function EnvironmentTab({ onSwitchTab }: EnvironmentTabProps) {
    const [search, setSearch] = useState("");

    const configQ = useQuery({
        queryKey: ["admin", "instance", "config"],
        queryFn: getInstanceConfig,
        retry: false,
    });

    const entries = useMemo(() => configQ.data?.entries ?? [], [configQ.data]);
    const filtered = useMemo(
        () => entries.filter((e) => matches(e, search.trim())),
        [entries, search],
    );

    const groups = useMemo(() => {
        const byGroup = new Map<string, InstanceConfigEntry[]>();
        for (const entry of filtered) {
            const list = byGroup.get(entry.group);
            if (list) list.push(entry);
            else byGroup.set(entry.group, [entry]);
        }
        return [...byGroup.entries()].sort(
            (a, b) => groupRank(a[0]) - groupRank(b[0]) || a[0].localeCompare(b[0]),
        );
    }, [filtered]);

    return (
        <div>
            <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
                <p className="max-w-2xl text-[13px] leading-relaxed text-muted-foreground">
                    Every value here comes from the environment of the running backend. Change it
                    where you set your environment, then restart. What can be edited in place
                    lives under Settings.
                </p>
                <Button size="sm" variant="outline" onClick={() => onSwitchTab?.("settings")}>
                    <Settings2 />
                    Instance settings
                </Button>
            </div>

            <div className="mb-8 flex flex-wrap items-center gap-3">
                <div className="relative w-full max-w-sm">
                    <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-subtle-foreground" />
                    <Input
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder="Search variables, groups or effects"
                        className="pl-8"
                    />
                </div>
                {entries.length > 0 && (
                    <span className="text-xs text-muted-foreground tabular-nums">
                        {filtered.length} of {entries.length} variables
                    </span>
                )}
            </div>

            {configQ.isLoading && (
                <div className="space-y-10">
                    {[0, 1].map((i) => (
                        <div key={i}>
                            <Skeleton className="mb-3 h-4 w-28" />
                            <Skeleton className="h-40 w-full rounded-lg" />
                        </div>
                    ))}
                </div>
            )}

            {configQ.isError && (
                <ErrorState
                    error={configQ.error}
                    title="Could not read this instance's configuration"
                    onRetry={() => configQ.refetch()}
                />
            )}

            {configQ.data && entries.length === 0 && (
                <div className="rounded-lg border border-dashed border-border">
                    <EmptyState icon={Terminal} title="The backend returned no configuration entries." />
                </div>
            )}

            {configQ.data && entries.length > 0 && filtered.length === 0 && (
                <div className="rounded-lg border border-dashed border-border">
                    <EmptyState icon={Search} title={`No variable matches "${search.trim()}".`} />
                </div>
            )}

            {groups.map(([group, groupEntries]) => (
                <SettingsGroup
                    key={group}
                    title={groupLabel(group)}
                    actions={
                        <span className="text-xs text-muted-foreground tabular-nums">
                            {groupEntries.length}
                        </span>
                    }
                >
                    {groupEntries.map((entry) => (
                        <ConfigRow key={entry.key} entry={entry} />
                    ))}
                </SettingsGroup>
            ))}
        </div>
    );
}

function ConfigRow({ entry }: { entry: InstanceConfigEntry }) {
    const source = SOURCE_STYLES[entry.source] ?? SOURCE_STYLES.default;
    const restart = RESTART_STYLES[entry.runtime_changeable] ?? RESTART_STYLES["boot-only"];

    return (
        // Anchored on the variable name so a check can deep-link to its row.
        <div id={entry.key} className="scroll-mt-20 px-4 py-3.5 target:bg-[var(--admin-accent-weak)]">
            <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5">
                <code className="min-w-0 break-all font-mono text-[12.5px] font-medium text-foreground">
                    {entry.key}
                </code>
                <div className="flex flex-wrap items-center gap-1.5">
                    <StatusBadge tone={source.tone} title={source.title}>
                        {source.label}
                    </StatusBadge>
                    <StatusBadge tone={restart.tone}>{restart.label}</StatusBadge>
                </div>
            </div>

            <div className="mt-2">
                {entry.sensitive ? (
                    <SensitiveValue entry={entry} />
                ) : (
                    <PlainValue entry={entry} />
                )}
            </div>

            {entry.effect && (
                <p className="mt-2 text-[12.5px] leading-relaxed text-muted-foreground">
                    {entry.effect}
                </p>
            )}

            {entry.docs && (
                <a
                    href={docsUrl(entry.docs)}
                    target="_blank"
                    rel="noreferrer"
                    className="mt-1.5 inline-flex items-center gap-1 text-xs font-medium text-[var(--admin-accent-strong)] hover:underline"
                >
                    Documentation
                    <ExternalLink className="size-3" />
                </a>
            )}
        </div>
    );
}

// Gated on the resolved value, not on entry.set: a default or derived value
// resolves without any environment variable being present.
function PlainValue({ entry }: { entry: InstanceConfigEntry }) {
    if (entry.value === "") {
        return <span className="text-xs text-subtle-foreground">Not set</span>;
    }
    return (
        <code className="block break-all rounded-md border border-border/70 bg-muted/60 px-2 py-1 font-mono text-xs text-foreground">
            {entry.value}
        </code>
    );
}

// A sensitive value is never sent. The fingerprint is there so two services
// can be compared (same AUTH_SECRET?) without disclosing either.
function SensitiveValue({ entry }: { entry: InstanceConfigEntry }) {
    // A fingerprint is only minted for a non-empty resolved value, so it is the
    // reliable "has a value" signal; source covers a backend that omits it.
    const resolved = entry.fingerprint !== "" || entry.source !== "unset";
    if (!resolved) {
        return <span className="text-xs text-subtle-foreground">Not set</span>;
    }
    return (
        <span className="inline-flex flex-wrap items-center gap-2">
            <span className="inline-flex items-center gap-1.5 rounded-md border border-border/70 bg-muted/60 px-2 py-1 font-mono text-xs text-muted-foreground">
                <Lock className="size-3 text-subtle-foreground" />
                Set, value hidden
            </span>
            {entry.fingerprint && (
                <span
                    className="font-mono text-[11px] text-muted-foreground"
                    title="First 4 hex characters of the SHA-256 of the value. Two services holding the same secret show the same fingerprint."
                >
                    fingerprint {entry.fingerprint}
                </span>
            )}
        </span>
    );
}
