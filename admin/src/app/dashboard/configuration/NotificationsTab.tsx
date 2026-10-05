// Operator notification channels: where this deployment tells its operator
// that something happened. Discord, Slack, a generic signed webhook, or email.
//
// The channels live in the same instance settings document as the rest of the
// writable configuration, so this page reads and writes /admin/instance/settings
// and only reaches for its own endpoints for the event catalog and the test
// delivery probe.
//
// Targets and secrets come back redacted. An unchanged field is sent back as
// the preview the server returned (or empty) and resolves to the stored value,
// so saving an unrelated toggle can never wipe a credential.

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
    Bell,
    Check,
    Hash,
    Mail,
    Plus,
    Save,
    Send,
    Trash2,
    Webhook,
} from "lucide-react";
import { ErrorState } from "@/components/ErrorState";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { EmptyState, Segmented, StatusBadge } from "@/components/ui/kit";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { TONE_TEXT } from "@/lib/tones";
import {
    getInstanceSettings,
    getNotificationEvents,
    putInstanceSettings,
    testNotificationChannel,
    type InstanceSettings,
    type NotifyChannel,
    type NotifyChannelType,
    type NotifyEventDef,
} from "@/lib/api/client/admin/instance";
import { SettingsRow } from "./SettingsLayout";

const SETTINGS_KEY = ["admin", "instance", "settings"];
const EVENTS_KEY = ["admin", "instance", "notification-events"];

const TYPES: {
    value: NotifyChannelType;
    label: string;
    icon: typeof Hash;
    placeholder: string;
    help: string;
}[] = [
    {
        value: "discord",
        label: "Discord",
        icon: Hash,
        placeholder: "https://discord.com/api/webhooks/…",
        help: "Server settings, then Integrations, then New Webhook. Copy the webhook URL.",
    },
    {
        value: "slack",
        label: "Slack",
        icon: Hash,
        placeholder: "https://hooks.slack.com/services/…",
        help: "Create a Slack app with an incoming webhook and copy its URL.",
    },
    {
        value: "webhook",
        label: "Webhook",
        icon: Webhook,
        placeholder: "https://example.com/hooks/warmbly",
        help: "Receives the event as JSON. Set a secret to have it signed with HMAC-SHA256.",
    },
    {
        value: "email",
        label: "Email",
        icon: Mail,
        placeholder: "ops@example.com",
        help: "Needs a working platform mail transport on this deployment.",
    },
];

function typeDef(t: NotifyChannelType) {
    return TYPES.find((x) => x.value === t) ?? TYPES[0];
}

function newChannel(): NotifyChannel {
    return {
        // The server assigns a real id on save; this one only has to be unique
        // within the unsaved list so React can key it.
        id: `new-${Math.random().toString(36).slice(2, 10)}`,
        name: "",
        type: "discord",
        target: "",
        secret: "",
        events: [],
        enabled: true,
    };
}

interface NotificationsTabProps {
    // Reported on every change so the page can confirm before a tab switch or
    // a navigation throws the edits away.
    onDirtyChange?: (dirty: boolean) => void;
}

export function NotificationsTab({ onDirtyChange }: NotificationsTabProps) {
    const queryClient = useQueryClient();
    const settings = useQuery({ queryKey: SETTINGS_KEY, queryFn: getInstanceSettings });
    const catalog = useQuery({ queryKey: EVENTS_KEY, queryFn: getNotificationEvents });

    const [channels, setChannels] = useState<NotifyChannel[] | null>(null);
    const [testing, setTesting] = useState<string | null>(null);

    useEffect(() => {
        if (settings.data) {
            setChannels(settings.data.notifications?.channels ?? []);
        }
    }, [settings.data]);

    const save = useMutation({
        mutationFn: (next: NotifyChannel[]) => {
            const base = settings.data as InstanceSettings;
            return putInstanceSettings({
                ...base,
                notifications: { channels: next },
            });
        },
        onSuccess: (doc) => {
            queryClient.setQueryData(SETTINGS_KEY, doc);
            setChannels(doc.notifications?.channels ?? []);
            toast.success("Notification channels saved");
        },
        onError: (e: Error) => toast.error(e.message || "Could not save channels"),
    });

    const groups = useMemo(() => {
        const events = catalog.data?.events ?? [];
        const order: string[] = [];
        const byGroup = new Map<string, NotifyEventDef[]>();
        for (const e of events) {
            if (!byGroup.has(e.group)) {
                byGroup.set(e.group, []);
                order.push(e.group);
            }
            byGroup.get(e.group)!.push(e);
        }
        return order.map((g) => ({ group: g, events: byGroup.get(g)! }));
    }, [catalog.data]);

    function update(id: string, patch: Partial<NotifyChannel>) {
        setChannels((prev) => (prev ?? []).map((c) => (c.id === id ? { ...c, ...patch } : c)));
    }

    function remove(id: string) {
        setChannels((prev) => (prev ?? []).filter((c) => c.id !== id));
    }

    function toggleEvent(id: string, key: string, on: boolean) {
        setChannels((prev) =>
            (prev ?? []).map((c) => {
                if (c.id !== id) return c;
                const next = on ? [...c.events, key] : c.events.filter((e) => e !== key);
                return { ...c, events: next };
            }),
        );
    }

    async function sendTest(ch: NotifyChannel) {
        setTesting(ch.id);
        try {
            // A saved channel is tested by id so the server uses its stored
            // credential; an unsaved one carries its fields inline.
            const saved = !ch.id.startsWith("new-");
            await testNotificationChannel(
                saved
                    ? { id: ch.id }
                    : { type: ch.type, name: ch.name, target: ch.target, secret: ch.secret },
            );
            toast.success("Test alert delivered");
        } catch (e) {
            toast.error((e as Error).message || "Delivery failed");
        } finally {
            setTesting(null);
        }
    }

    const list = channels ?? [];
    const dirty =
        !!settings.data &&
        JSON.stringify(list) !== JSON.stringify(settings.data.notifications?.channels ?? []);

    useEffect(() => {
        onDirtyChange?.(dirty);
        return () => onDirtyChange?.(false);
    }, [dirty, onDirtyChange]);

    if (settings.isError) {
        return <ErrorState error={settings.error as Error} onRetry={() => settings.refetch()} />;
    }

    // Changing a channel's type clears its target on purpose, so a save with
    // one still empty would drop the channel server-side. Block it here and
    // say which one needs attention.
    const incomplete = list.filter((c) => !c.target.trim());

    return (
        <div>
            <div className="mb-8 flex flex-wrap items-start justify-between gap-3">
                <p className="max-w-2xl text-[13px] leading-relaxed text-muted-foreground">
                    Where this instance tells you something happened. Add a Discord or Slack
                    webhook, a signed endpoint, or an address.
                </p>
                <div className="flex items-center gap-1.5">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setChannels([...(channels ?? []), newChannel()])}
                    >
                        <Plus />
                        Add channel
                    </Button>
                    <Button
                        size="sm"
                        disabled={!dirty || incomplete.length > 0 || save.isPending}
                        title={
                            incomplete.length > 0
                                ? "Every channel needs a destination before you can save"
                                : undefined
                        }
                        onClick={() => save.mutate(list)}
                    >
                        <Save />
                        {save.isPending ? "Saving…" : "Save"}
                    </Button>
                </div>
            </div>

            {settings.isLoading ? (
                <Skeleton className="h-64 w-full rounded-lg" />
            ) : list.length === 0 ? (
                <div className="rounded-lg border border-dashed border-border">
                    <EmptyState
                        icon={Bell}
                        title="No channels yet"
                        hint="Nothing is being sent anywhere. Add a channel and pick the events it should receive; leave every event unchecked to receive all of them."
                        action={
                            <Button size="sm" onClick={() => setChannels([newChannel()])}>
                                <Plus />
                                Add a channel
                            </Button>
                        }
                    />
                </div>
            ) : (
                <div className="space-y-6">
                    {list.map((ch) => {
                        const def = typeDef(ch.type);
                        const Icon = def.icon;
                        const saved = !ch.id.startsWith("new-");
                        return (
                            <section
                                key={ch.id}
                                className="overflow-hidden surface-lit rounded-xl border border-border bg-card"
                            >
                                <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
                                    <div className="flex min-w-0 items-center gap-3">
                                        <span className="grid size-8 shrink-0 place-items-center rounded-md border border-border bg-muted/50 text-muted-foreground">
                                            <Icon className="size-4" />
                                        </span>
                                        <div className="min-w-0">
                                            <div className="truncate text-[13px] font-medium text-foreground">
                                                {ch.name || def.label}
                                            </div>
                                            <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                                                {ch.events.length === 0
                                                    ? "Receives every event"
                                                    : `Receives ${ch.events.length} event${ch.events.length === 1 ? "" : "s"}`}
                                                {!saved && (
                                                    <>
                                                        <span className="text-subtle-foreground">·</span>
                                                        <span className={TONE_TEXT.warning}>unsaved</span>
                                                    </>
                                                )}
                                            </div>
                                        </div>
                                    </div>
                                    <div className="flex items-center gap-1.5">
                                        {!ch.enabled && <StatusBadge>Off</StatusBadge>}
                                        <Switch
                                            checked={ch.enabled}
                                            onCheckedChange={(v) => update(ch.id, { enabled: v })}
                                            className="mx-1.5"
                                        />
                                        <Button
                                            variant="outline"
                                            size="sm"
                                            disabled={testing === ch.id || !ch.target}
                                            onClick={() => sendTest(ch)}
                                        >
                                            <Send />
                                            {testing === ch.id ? "Sending…" : "Test"}
                                        </Button>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            aria-label="Remove channel"
                                            onClick={() => remove(ch.id)}
                                        >
                                            <Trash2 />
                                        </Button>
                                    </div>
                                </div>

                                <div className="divide-y divide-border">
                                    <SettingsRow label="Type">
                                        <Segmented
                                            ariaLabel="Channel type"
                                            value={ch.type}
                                            onChange={(t) => update(ch.id, { type: t, target: "" })}
                                            options={TYPES.map((t) => ({ value: t.value, label: t.label }))}
                                        />
                                    </SettingsRow>
                                    <SettingsRow label="Name" htmlFor={`name-${ch.id}`}>
                                        <Input
                                            id={`name-${ch.id}`}
                                            value={ch.name}
                                            placeholder={def.label}
                                            onChange={(e) => update(ch.id, { name: e.target.value })}
                                            className="sm:w-80"
                                        />
                                    </SettingsRow>
                                    <SettingsRow
                                        label={ch.type === "email" ? "Address" : "Webhook URL"}
                                        htmlFor={`target-${ch.id}`}
                                        description={ch.target.trim() ? def.help : undefined}
                                        error={
                                            ch.target.trim()
                                                ? undefined
                                                : ch.type === "email"
                                                  ? "Enter an address before saving."
                                                  : "Enter the webhook URL for this transport before saving."
                                        }
                                    >
                                        <Input
                                            id={`target-${ch.id}`}
                                            value={ch.target}
                                            placeholder={def.placeholder}
                                            onChange={(e) => update(ch.id, { target: e.target.value })}
                                            className="sm:w-80"
                                        />
                                    </SettingsRow>

                                    {ch.type === "webhook" && (
                                        <SettingsRow
                                            label="Signing secret"
                                            htmlFor={`secret-${ch.id}`}
                                            description={
                                                <>
                                                    Signs the body as{" "}
                                                    <code className="rounded bg-muted px-1 py-px font-mono text-[11.5px] text-foreground">
                                                        X-Warmbly-Signature: t=&lt;unix&gt;,v1=&lt;hex&gt;
                                                    </code>
                                                    , the same scheme customer webhooks use.
                                                </>
                                            }
                                        >
                                            <Input
                                                id={`secret-${ch.id}`}
                                                value={ch.secret ?? ""}
                                                placeholder="Optional"
                                                data-ph-mask=""
                                                onChange={(e) => update(ch.id, { secret: e.target.value })}
                                                className="sm:w-80"
                                            />
                                        </SettingsRow>
                                    )}

                                    <div className="px-4 py-3.5">
                                        <div className="flex items-center justify-between gap-3">
                                            <div className="text-[13px] font-medium text-foreground">Events</div>
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="xs"
                                                onClick={() => update(ch.id, { events: [] })}
                                            >
                                                Receive everything
                                            </Button>
                                        </div>
                                        {catalog.isLoading ? (
                                            <Skeleton className="mt-3 h-24 w-full" />
                                        ) : (
                                            <div className="mt-3 grid gap-x-6 gap-y-4 md:grid-cols-2">
                                                {groups.map(({ group, events }) => (
                                                    <div key={group}>
                                                        <p className="mb-1 text-xs font-medium text-muted-foreground">
                                                            {group}
                                                        </p>
                                                        <div className="-mx-2">
                                                            {events.map((e) => (
                                                                <label
                                                                    key={e.key}
                                                                    className="flex cursor-pointer items-start gap-2.5 rounded-md px-2 py-1.5 transition-colors hover:bg-accent/50"
                                                                >
                                                                    <Checkbox
                                                                        checked={ch.events.includes(e.key)}
                                                                        onCheckedChange={(v) =>
                                                                            toggleEvent(ch.id, e.key, v === true)
                                                                        }
                                                                        className="mt-0.5"
                                                                    />
                                                                    <span className="text-[13px] leading-snug text-foreground">
                                                                        {e.label}
                                                                        <span className="mt-0.5 block text-xs text-muted-foreground">
                                                                            {e.description}
                                                                        </span>
                                                                    </span>
                                                                </label>
                                                            ))}
                                                        </div>
                                                    </div>
                                                ))}
                                            </div>
                                        )}
                                        {ch.events.length === 0 && (
                                            <p className="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
                                                <Check className={`size-3 ${TONE_TEXT.success}`} />
                                                Nothing selected, so this channel receives every event.
                                            </p>
                                        )}
                                    </div>
                                </div>
                            </section>
                        );
                    })}
                </div>
            )}
        </div>
    );
}
