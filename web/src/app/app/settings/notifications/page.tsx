import React from "react";
import {
    useNotificationPreferences,
    useUpdateNotificationPreferences,
} from "@/lib/api/hooks/app/notifications/useNotifications";
import {
    EMAIL_WINDOW_MAX_MINUTES,
    EMAIL_WINDOW_MIN_MINUTES,
    NOTIFICATION_CATEGORY_GROUPS,
    NOTIFICATION_CATEGORY_KEYS as CATEGORY_KEYS,
    normalizeNotificationPreferences,
    type NotificationCategoryDef,
    type NotificationCategoryKey,
    type NotificationPreferences,
} from "@/lib/api/models/app/notifications/Notification";
import { Row, Section, SectionShell, Toggle } from "../_components/SectionShell";
import { OptionSelect } from "@/components/app/campaigns/preferences/components/CampaignPreferenceBoolBox";
import { NumberInput } from "@/components/ui/field";
import SaveStatus from "../_components/SaveStatus";
import { useAutosave } from "@/hooks/useAutosave";
import { useRegisterUnsaved } from "@/hooks/context/unsaved";

// Section copy per category group; the groups themselves live with the model.
const GROUP_DESCRIPTIONS: Record<string, string> = {
    inbound: "Get notified about replies on a campaign you're running. Off by default to keep high-volume sends quiet.",
    health: "Deliverability + infrastructure alerts. Recommended on.",
    security: "Account access alerts.",
    billing: "Trial and billing alerts.",
    team: "Activity from your teammates.",
};

// Window presets in minutes; "custom" reveals a minutes input. There is no
// per-event option on purpose — 30 minutes is the floor.
const WINDOW_PRESETS: { value: string; label: React.ReactNode; hint: string }[] = [
    { value: "30", label: "Every 30 minutes", hint: "The fastest option. Bundles anything you have not already read." },
    { value: "60", label: "Every hour", hint: "At most one bundled email per hour." },
    { value: "180", label: "Every 3 hours", hint: "A few bundles across a working day." },
    { value: "1440", label: "Once a day", hint: "One daily summary of everything unread." },
    { value: "custom", label: "Custom", hint: "Pick your own window, from 30 minutes up to a day." },
];

export default function NotificationsSettingsPage() {
    const { data, isLoading } = useNotificationPreferences();
    const update = useUpdateNotificationPreferences();
    const [draft, setDraft] = React.useState<NotificationPreferences | null>(null);

    // Auto-save: toggles persist instantly. markSaved on data load moves the
    // baseline to the server value so the initial null→data hydration (and any
    // refetch) is never mistaken for a user edit.
    const autosave = useAutosave({
        value: draft,
        enabled: !!draft,
        save: async (v) => {
            if (v) await update.mutateAsync(v);
        },
    });
    useRegisterUnsaved(autosave, () => setDraft(autosave.savedValue));

    // One-shot hydration: server data seeds the draft once (normalized, since
    // an older backend or cached response may miss newer categories the rows
    // index directly). After that the save path owns the baseline — re-adopting
    // every refetch would stomp edits made while a save was in flight.
    const hydratedRef = React.useRef(false);
    React.useEffect(() => {
        if (!data || hydratedRef.current) return;
        hydratedRef.current = true;
        const full = normalizeNotificationPreferences(data.preferences);
        setDraft(full);
        autosave.markSaved(full);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [data]);

    // Window selection: preset when the minutes match one, custom otherwise.
    // customMode keeps Custom selected while its input holds a preset value.
    const [customMode, setCustomMode] = React.useState(false);
    const minutes = draft?.email_digest_minutes ?? EMAIL_WINDOW_MIN_MINUTES;
    const matchingPreset = WINDOW_PRESETS.find((p) => p.value === String(minutes) && p.value !== "custom");
    const windowSelection = customMode || !matchingPreset ? "custom" : matchingPreset.value;
    const setMinutes = (m: number) =>
        setDraft((d) => (d ? { ...d, email_digest_minutes: m } : d));
    const pickWindow = (v: string) => {
        if (v === "custom") {
            setCustomMode(true);
            return;
        }
        setCustomMode(false);
        setMinutes(Number(v));
    };

    const setEnabled = (key: NotificationCategoryKey, on: boolean) =>
        setDraft((d) => (d ? { ...d, [key]: { ...d[key], enabled: on } } : d));

    // Channels present globally: "on" when every category carries the channel.
    const channelOn = (ch: "email" | "slack" | "push") =>
        !!draft && CATEGORY_KEYS.every((k) => draft[k].channels[ch]);
    const setChannel = (ch: "email" | "slack" | "push", on: boolean) =>
        setDraft((d) => {
            if (!d) return d;
            const next = { ...d };
            for (const k of CATEGORY_KEYS) {
                next[k] = { ...d[k], channels: { ...d[k].channels, [ch]: on } };
            }
            return next;
        });

    const rows = (items: NotificationCategoryDef[]) =>
        items.map((c) => (
            <Row key={c.key} label={c.label} description={c.hint}>
                <Toggle on={!!draft && draft[c.key].enabled} onChange={(v) => setEnabled(c.key, v)} />
            </Row>
        ));

    return (
        <SectionShell
            title="Notifications"
            description="Which events notify you, and where they are delivered. Defaults reflect the recommendation."
            actions={<SaveStatus status={autosave.status} onRetry={autosave.retry} />}
        >
            {isLoading || !draft ? (
                <div className="px-5 py-10 text-[12.5px] text-slate-400">Loading…</div>
            ) : (
                <>
                    {NOTIFICATION_CATEGORY_GROUPS.map((g) => (
                        <Section key={g.id} eyebrow={g.label} description={GROUP_DESCRIPTIONS[g.id]}>
                            {rows(g.categories)}
                        </Section>
                    ))}
                    <Section eyebrow="Channels" description="Where enabled notifications are delivered. Applies across every category above.">
                        <Row label="In-app" description="The bell in the dashboard chrome (controlled per category above).">
                            <span className="text-[11px] font-medium text-emerald-600">On</span>
                        </Row>
                        <Row
                            label="Mobile push"
                            description="Alerts on devices signed in with the Warmbly iOS app. The first event pushes right away; bursts arrive as one summary instead of a ping per event."
                        >
                            <Toggle on={channelOn("push")} onChange={(v) => setChannel("push", v)} />
                        </Row>
                        <Row label="Email" description="Delivery to your account email.">
                            <Toggle on={channelOn("email")} onChange={(v) => setChannel("email", v)} />
                        </Row>
                        <Row
                            label="Slack"
                            description="Posts to the channel your workspace routes each category to in the Slack integration, and to your Slack DMs when you link your Slack account and turn on DM notifications there."
                        >
                            <Toggle on={channelOn("slack")} onChange={(v) => setChannel("slack", v)} />
                        </Row>
                    </Section>
                    <Section eyebrow="Email delivery" description="How often the email channel sends. Everything unread bundles into one email per window, so a busy day costs a few emails instead of one per alert.">
                        <OptionSelect
                            aria-label="Email bundling window"
                            cols={2}
                            value={windowSelection}
                            onChange={pickWindow}
                            options={WINDOW_PRESETS}
                        />
                        {windowSelection === "custom" && (
                            <div className="flex items-center gap-2">
                                <span className="text-[12px] text-slate-500">Bundle every</span>
                                <NumberInput
                                    value={minutes}
                                    onChange={setMinutes}
                                    min={EMAIL_WINDOW_MIN_MINUTES}
                                    max={EMAIL_WINDOW_MAX_MINUTES}
                                    step={15}
                                    suffix="minutes"
                                    className="w-36"
                                />
                            </div>
                        )}
                        <p className="text-[11px] text-slate-400 leading-relaxed">
                            Alerts you read in the app are never emailed. Security sign-in alerts always send immediately, and alerts that concern several teammates arrive as one shared email.
                        </p>
                    </Section>
                </>
            )}
        </SectionShell>
    );
}
