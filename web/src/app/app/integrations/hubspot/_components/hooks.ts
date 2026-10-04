import React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import toast from "react-hot-toast";

import { useConfirm } from "@/hooks/context/confirm";
import {
    useFinishIntegrationOAuth,
    useReauthIntegration,
    useStartIntegrationOAuth,
} from "@/lib/api/hooks/app/integrations/useIntegrationOAuth";
import type { IntegrationConnection } from "@/lib/api/models/app/integrations/Integration";
import { openOAuthPopup } from "@/lib/integrations/oauthPopup";
import { stringifySearch } from "@/lib/routerSearch";
import usePipelines from "@/lib/api/hooks/app/crm/pipelines/usePipelines";
import type Pipeline from "@/lib/api/models/app/crm/Pipeline";

import { errMessage } from "./shared";

// The HubSpot OAuth popup: a first connect, or a reconnect that grants the
// permissions CRM mode needs.
export function useHubSpotOAuth() {
    const queryClient = useQueryClient();
    const start = useStartIntegrationOAuth();
    const finish = useFinishIntegrationOAuth();
    const reauth = useReauthIntegration();
    const [busy, setBusy] = React.useState(false);

    const connect = React.useCallback(async (): Promise<IntegrationConnection | null> => {
        setBusy(true);
        try {
            const { url } = await start.mutateAsync({ provider: "hubspot" });
            const { code, state } = await openOAuthPopup(url);
            const conn = await finish.mutateAsync({ code, state });
            toast.success("HubSpot connected");
            return conn;
        } catch (err) {
            toast.error(errMessage(err, "Could not connect HubSpot"));
            return null;
        } finally {
            setBusy(false);
        }
    }, [start, finish]);

    const reconnect = React.useCallback(
        async (connectionId: string): Promise<boolean> => {
            setBusy(true);
            try {
                const { url } = await reauth.mutateAsync(connectionId);
                const { code, state } = await openOAuthPopup(url);
                await finish.mutateAsync({ code, state });
                await queryClient.invalidateQueries({ queryKey: ["crm"] });
                toast.success("HubSpot reconnected");
                return true;
            } catch (err) {
                toast.error(errMessage(err, "Could not reconnect HubSpot"));
                return false;
            } finally {
                setBusy(false);
            }
        },
        [reauth, finish, queryClient],
    );

    return { connect, reconnect, busy };
}

// Asks before an in-app navigation or a reload throws away unsaved choices.
export function useLeaveGuard(dirty: boolean, message: string) {
    const confirm = useConfirm();
    const navigate = useNavigate();
    const allow = React.useRef(false);

    const blocker = useBlocker({
        shouldBlockFn: React.useCallback(
            ({ current, next }: { current: { pathname: string }; next: { pathname: string } }) =>
                dirty && !allow.current && current.pathname !== next.pathname,
            [dirty],
        ),
        withResolver: true,
        enableBeforeUnload: false,
    });

    React.useEffect(() => {
        if (blocker.status !== "blocked") return;
        const to = blocker.next;
        blocker.reset();
        confirm.show(message, async () => {
            allow.current = true;
            navigate({ href: `${to.pathname}${stringifySearch(to.search)}` });
        });
    }, [blocker, confirm, message, navigate]);

    React.useEffect(() => {
        if (!dirty) return;
        const handler = (e: BeforeUnloadEvent) => {
            e.preventDefault();
            e.returnValue = "";
        };
        window.addEventListener("beforeunload", handler);
        return () => window.removeEventListener("beforeunload", handler);
    }, [dirty]);

    // Lets a deliberate exit (finish, cancel) leave without asking.
    return React.useCallback(() => {
        allow.current = true;
    }, []);
}

// HubSpot pipelines mirrored into Warmbly; everything else is Warmbly's own.
export function useHubSpotPipelines(): { pipelines: Pipeline[]; loading: boolean } {
    const q = usePipelines();
    const all = q.data ?? [];
    const mirrored = all.filter((p) => p.external?.provider === "hubspot");
    return { pipelines: mirrored.length ? mirrored : all, loading: q.isLoading };
}
