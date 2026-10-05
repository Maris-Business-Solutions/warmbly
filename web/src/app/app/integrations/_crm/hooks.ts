import React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import toast from "react-hot-toast";

import { useConfirm } from "@/hooks/context/confirm";
import {
    useFinishIntegrationOAuth,
    useReauthIntegration,
    useStartIntegrationOAuth,
} from "@/lib/api/hooks/app/integrations/useIntegrationOAuth";
import type { IntegrationConnection } from "@/lib/api/models/app/integrations/Integration";
import { authorizeInPopup } from "@/lib/integrations/oauthPopup";
import usePipelines from "@/lib/api/hooks/app/crm/pipelines/usePipelines";
import type Pipeline from "@/lib/api/models/app/crm/Pipeline";

import { usePageCrm } from "./context";
import { errMessage } from "./shared";

// The provider's OAuth popup: a first connect, or a reconnect that grants the
// permissions CRM mode needs.
export function useCrmOAuth() {
    const crm = usePageCrm();
    const queryClient = useQueryClient();
    const start = useStartIntegrationOAuth();
    const finish = useFinishIntegrationOAuth();
    const reauth = useReauthIntegration();
    const [busy, setBusy] = React.useState(false);

    const connect = React.useCallback(async (): Promise<IntegrationConnection | null> => {
        setBusy(true);
        try {
            const { code, state } = await authorizeInPopup(async () => (await start.mutateAsync({ provider: crm.id })).url);
            const conn = await finish.mutateAsync({ code, state });
            toast.success(`${crm.name} connected`);
            return conn;
        } catch (err) {
            toast.error(errMessage(err, `Could not connect ${crm.name}`));
            return null;
        } finally {
            setBusy(false);
        }
    }, [start, finish, crm]);

    const reconnect = React.useCallback(
        async (connectionId: string): Promise<boolean> => {
            setBusy(true);
            try {
                const { code, state } = await authorizeInPopup(async () => (await reauth.mutateAsync(connectionId)).url);
                await finish.mutateAsync({ code, state });
                await queryClient.invalidateQueries({ queryKey: ["crm"] });
                toast.success(`${crm.name} reconnected`);
                return true;
            } catch (err) {
                toast.error(errMessage(err, `Could not reconnect ${crm.name}`));
                return false;
            } finally {
                setBusy(false);
            }
        },
        [reauth, finish, queryClient, crm],
    );

    return { connect, reconnect, busy };
}

// Asks before an in-app navigation or a reload throws away unsaved choices.
export function useLeaveGuard(dirty: boolean, message: string) {
    const confirm = useConfirm();
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

    // The navigation stays held while the dialog is open, so confirming resumes it exactly (Back stays Back).
    React.useEffect(() => {
        if (blocker.status !== "blocked") return;
        confirm.show(
            message,
            async () => {
                allow.current = true;
                blocker.proceed();
            },
            () => blocker.reset(),
        );
    }, [blocker, confirm, message]);

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

// The page's CRM pipelines mirrored into Warmbly; the rest are Warmbly's own.
export function useProviderPipelines(): { pipelines: Pipeline[]; loading: boolean } {
    const crm = usePageCrm();
    const q = usePipelines();
    const all = q.data ?? [];
    const mirrored = all.filter((p) => p.external?.provider === crm.id);
    return { pipelines: mirrored.length ? mirrored : all, loading: q.isLoading };
}
