import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
    confirmSlackLink,
    deleteMySlackLink,
    deleteSlackLink,
    getSlackLinkPreview,
    getSlackStatus,
    listSlackChannels,
    updateMySlackLink,
    updateSlackSettings,
} from "@/lib/api/client/app/integrations/slack";
import type { SlackStatus } from "@/lib/api/models/app/integrations/Slack";

// Everything Slack lives under this key; the audit spine's `integration`
// entry invalidates it, so a teammate's change lands live.
export const SLACK_KEY = ["integrations", "slack"] as const;
const STATUS_KEY = [...SLACK_KEY, "status"] as const;

export function useSlackStatus(enabled = true) {
    return useQuery({
        queryKey: STATUS_KEY,
        queryFn: getSlackStatus,
        enabled,
        staleTime: 15_000,
    });
}

// Channel search for the picker. The caller debounces `q`.
export function useSlackChannels(q: string, enabled: boolean) {
    return useQuery({
        queryKey: [...SLACK_KEY, "channels", q],
        queryFn: ({ signal }) => listSlackChannels(q, signal),
        enabled,
        placeholderData: keepPreviousData,
        staleTime: 60_000,
    });
}

export function useUpdateSlackSettings() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: updateSlackSettings,
        onSuccess: (settings) => {
            qc.setQueryData<SlackStatus>(STATUS_KEY, (s) => (s ? { ...s, settings } : s));
            qc.invalidateQueries({ queryKey: STATUS_KEY });
        },
    });
}

export function useSlackLinkPreview(code: string) {
    return useQuery({
        queryKey: [...SLACK_KEY, "link", code],
        queryFn: () => getSlackLinkPreview(code),
        enabled: code !== "",
        retry: false,
        staleTime: Infinity,
    });
}

function useSlackMutation<TVars, TResult>(fn: (v: TVars) => Promise<TResult>) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: fn,
        onSuccess: () => {
            qc.invalidateQueries({ queryKey: STATUS_KEY });
        },
    });
}

export function useConfirmSlackLink() {
    return useSlackMutation(confirmSlackLink);
}

export function useUpdateMySlackLink() {
    return useSlackMutation(updateMySlackLink);
}

export function useDeleteMySlackLink() {
    return useSlackMutation(() => deleteMySlackLink());
}

export function useDeleteSlackLink() {
    return useSlackMutation(deleteSlackLink);
}
