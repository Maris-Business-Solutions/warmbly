import type {
    SlackChannelList,
    SlackLinkPreview,
    SlackSettings,
    SlackStatus,
    SlackUserLink,
} from "@/lib/api/models/app/integrations/Slack";
import Request from "../../Request";

export async function getSlackStatus(): Promise<SlackStatus> {
    return await Request<SlackStatus>({
        method: "GET",
        url: "/integrations/slack/status",
        authorization: true,
    });
}

export async function listSlackChannels(q: string, signal?: AbortSignal): Promise<SlackChannelList> {
    const params = new URLSearchParams();
    if (q) params.set("q", q);
    const qs = params.toString();
    const result = await Request<SlackChannelList>({
        method: "GET",
        url: `/integrations/slack/channels${qs ? `?${qs}` : ""}`,
        authorization: true,
        signal,
    });
    return {
        data: result.data ?? [],
        pagination: result.pagination ?? { has_more: false, next_cursor: null },
    };
}

export async function updateSlackSettings(settings: SlackSettings): Promise<SlackSettings> {
    return await Request<SlackSettings>({
        method: "PUT",
        url: "/integrations/slack/settings",
        data: settings,
        authorization: true,
    });
}

export async function getSlackLinkPreview(code: string): Promise<SlackLinkPreview> {
    return await Request<SlackLinkPreview>({
        method: "GET",
        url: `/integrations/slack/link/${encodeURIComponent(code)}`,
        authorization: true,
    });
}

// slack_code and state are a Sign in with Slack result, for a link whose emails differ.
export async function confirmSlackLink(input: { code: string; slack_code?: string; state?: string }): Promise<SlackUserLink> {
    return await Request<SlackUserLink>({
        method: "POST",
        url: "/integrations/slack/link",
        data: input,
        authorization: true,
    });
}

export async function startSlackLinkVerify(code: string): Promise<{ url: string }> {
    return await Request<{ url: string }>({
        method: "POST",
        url: "/integrations/slack/link/verify",
        data: { code },
        authorization: true,
    });
}

export async function updateMySlackLink(input: { dm_notifications: boolean }): Promise<SlackUserLink> {
    return await Request<SlackUserLink>({
        method: "PATCH",
        url: "/integrations/slack/link",
        data: input,
        authorization: true,
    });
}

export async function deleteMySlackLink(): Promise<void> {
    await Request<void>({
        method: "DELETE",
        url: "/integrations/slack/link",
        authorization: true,
    });
}

export async function deleteSlackLink(id: string): Promise<void> {
    await Request<void>({
        method: "DELETE",
        url: `/integrations/slack/links/${id}`,
        authorization: true,
    });
}
