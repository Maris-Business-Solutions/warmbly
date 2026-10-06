import React from "react";
import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast/headless";
import {
    getViewPreferences,
    resetViewPreferences,
    updateViewPreferences,
} from "@/lib/api/client/app/views/views";
import type {
    UpdateViewPreferences,
    ViewName,
    ViewPreferences,
} from "@/lib/api/models/app/views/ViewPreferences";

// A layout belongs to one member in one workspace, so the cache is keyed by
// both: two accounts sharing a browser never see each other's columns.
export interface ViewScope {
    userId: string;
    orgId: string;
}

const queryKey = (scope: ViewScope, view: ViewName) => ["views", scope.userId, scope.orgId, view];
const cacheKey = (scope: ViewScope, view: ViewName) => `warmbly:view:${scope.userId}:${scope.orgId}:${view}`;
const scoped = (scope: ViewScope) => !!scope.userId && !!scope.orgId;

// The server is the source of truth; the browser keeps a copy so the list
// paints with the member's columns on the first frame instead of flashing the
// defaults while the request is in flight. Anything malformed is ignored.
export function readCachedView(scope: ViewScope, view: ViewName): ViewPreferences | undefined {
    if (!scoped(scope)) return undefined;
    try {
        const raw = localStorage.getItem(cacheKey(scope, view));
        if (!raw) return undefined;
        const p = JSON.parse(raw) as Partial<ViewPreferences> | null;
        if (!p || !Array.isArray(p.columns)) return undefined;
        const sort =
            p.sort && typeof p.sort.by === "string" && p.sort.by
                ? { by: p.sort.by, reverse: !!p.sort.reverse }
                : null;
        return { view, columns: p.columns.filter((c): c is string => typeof c === "string"), sort };
    } catch {
        return undefined;
    }
}

function writeCachedView(scope: ViewScope, view: ViewName, prefs: ViewPreferences | null) {
    if (!scoped(scope)) return;
    try {
        if (!prefs) localStorage.removeItem(cacheKey(scope, view));
        else localStorage.setItem(cacheKey(scope, view), JSON.stringify({ columns: prefs.columns, sort: prefs.sort ?? null }));
    } catch {
        /* storage full or blocked: the server copy still applies */
    }
}

export const viewPreferencesQuery = (view: ViewName, scope: ViewScope) =>
    queryOptions({
        queryKey: queryKey(scope, view),
        queryFn: async () => (await getViewPreferences(view)).preferences,
        staleTime: 5 * 60 * 1000,
    });

export function useViewPreferences(view: ViewName, scope: ViewScope) {
    const q = useQuery({
        ...viewPreferencesQuery(view, scope),
        enabled: scoped(scope),
        placeholderData: () => readCachedView(scope, view),
    });
    React.useEffect(() => {
        if (q.data && !q.isPlaceholderData) writeCachedView(scope, view, q.data);
    }, [q.data, q.isPlaceholderData, scope, view]);
    return q;
}

// Optimistic: the table re-lays out on the click, and the row is written
// behind it. The body is partial (columns, sort, or both) and the server keeps
// whatever it does not name, so a click made before the saved layout has
// loaded cannot erase the rest of it. A failure puts the previous layout back
// and says so; the server's reply is what stays in the cache.
export function useUpdateViewPreferences(view: ViewName, scope: ViewScope) {
    const qc = useQueryClient();
    const key = queryKey(scope, view);
    return useMutation({
        mutationFn: (body: UpdateViewPreferences) => updateViewPreferences(view, body),
        onMutate: async (body) => {
            await qc.cancelQueries({ queryKey: key });
            const previous = qc.getQueryData<ViewPreferences>(key);
            const next: ViewPreferences = {
                view,
                columns: body.columns ?? previous?.columns ?? [],
                sort: body.sort !== undefined ? (body.sort.by ? body.sort : null) : (previous?.sort ?? null),
            };
            qc.setQueryData<ViewPreferences>(key, next);
            writeCachedView(scope, view, next);
            return { previous };
        },
        onError: (_err, _body, ctx) => {
            qc.setQueryData(key, ctx?.previous);
            writeCachedView(scope, view, ctx?.previous ?? null);
            toast.error("Couldn't save your view. It will reset when you reload.");
        },
        onSuccess: (envelope) => {
            qc.setQueryData(key, envelope.preferences);
            writeCachedView(scope, view, envelope.preferences);
        },
    });
}

export function useResetViewPreferences(view: ViewName, scope: ViewScope) {
    const qc = useQueryClient();
    const key = queryKey(scope, view);
    return useMutation({
        mutationFn: () => resetViewPreferences(view),
        onMutate: async () => {
            await qc.cancelQueries({ queryKey: key });
            const previous = qc.getQueryData<ViewPreferences>(key);
            qc.setQueryData<ViewPreferences>(key, { view, columns: [], sort: null });
            writeCachedView(scope, view, null);
            return { previous };
        },
        onError: (_err, _v, ctx) => {
            qc.setQueryData(key, ctx?.previous);
            writeCachedView(scope, view, ctx?.previous ?? null);
            toast.error("Couldn't reset the view.");
        },
    });
}
