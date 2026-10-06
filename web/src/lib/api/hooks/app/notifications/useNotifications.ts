import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast/headless";
import {
    getNotificationPreferences,
    updateNotificationPreferences,
    listNotifications,
    markNotificationRead,
    markAllNotificationsRead,
} from "@/lib/api/client/app/notifications/notifications";
import type { AppNotification, NotificationPreferences } from "@/lib/api/models/app/notifications/Notification";
import { patchQueries, restoreQueries, settle } from "@/lib/api/hooks/optimistic";

const PREFS_KEY = ["notifications", "preferences"];
const FEED_KEY = ["notifications", "feed"];

export function useNotificationPreferences() {
    return useQuery({
        queryKey: PREFS_KEY,
        queryFn: getNotificationPreferences,
        staleTime: 60_000,
    });
}

// The PUT echoes the saved envelope; write it into the cache instead of
// invalidating — a refetch per save is wasted traffic and re-rendered the
// settings page mid-save.
export function useUpdateNotificationPreferences() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (prefs: NotificationPreferences) => updateNotificationPreferences(prefs),
        onSuccess: (envelope) => qc.setQueryData(PREFS_KEY, envelope),
    });
}

export function useNotifications(unreadOnly = false) {
    return useQuery({
        queryKey: [...FEED_KEY, { unreadOnly }],
        queryFn: () => listNotifications(unreadOnly, 50),
        staleTime: 15_000,
    });
}

type Feed = { notifications: AppNotification[]; unread: number };

const FEED_MUTATION = ["notifications", "read"];

// Marks the matching unread entries read and takes them off the badge count.
function markRead(match: (n: AppNotification) => boolean, all = false) {
    return (data: unknown) => {
        const feed = data as Feed | undefined;
        if (!Array.isArray(feed?.notifications)) return data;
        const now = new Date();
        let read = 0;
        const notifications = feed.notifications.map((n) => {
            if (n.read_at || !match(n)) return n;
            read++;
            return { ...n, read_at: now };
        });
        return { ...feed, notifications, unread: all ? 0 : Math.max(0, feed.unread - read) };
    };
}

// Optimistic: the dot and the badge clear on the click; a refusal puts them back.
export function useMarkNotificationRead() {
    const qc = useQueryClient();
    return useMutation({
        mutationKey: [...FEED_MUTATION, "one"],
        mutationFn: (id: string) => markNotificationRead(id),
        onMutate: (id) => patchQueries(qc, [FEED_KEY], markRead((n) => n.id === id)),
        onError: (_err, _id, snapshot) => restoreQueries(qc, snapshot),
        onSettled: () => settle(qc, FEED_MUTATION, [FEED_KEY]),
    });
}

export function useMarkAllNotificationsRead() {
    const qc = useQueryClient();
    return useMutation({
        mutationKey: [...FEED_MUTATION, "all"],
        mutationFn: () => markAllNotificationsRead(),
        onMutate: () => patchQueries(qc, [FEED_KEY], markRead(() => true, true)),
        onError: (_err, _vars, snapshot) => {
            restoreQueries(qc, snapshot);
            toast.error("Couldn't mark notifications as read");
        },
        onSettled: () => settle(qc, FEED_MUTATION, [FEED_KEY]),
    });
}
