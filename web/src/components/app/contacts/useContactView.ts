// The member's saved layout of a contacts list, resolved against the column
// registry and the workspace's custom fields. One hook, so the table, the
// chooser and the sort menu read the same view.

import React from "react";
import { useCurrentOrg, useUser } from "@/stores/useAppStore";
import useCustomFieldKeys from "@/lib/api/hooks/app/contacts/useCustomFieldKeys";
import {
    useResetViewPreferences,
    useUpdateViewPreferences,
    useViewPreferences,
    type ViewScope,
} from "@/lib/api/hooks/app/views/useViewPreferences";
import type { ColumnViewName, ViewSort } from "@/lib/api/models/app/views/ViewPreferences";
import { resolveColumns } from "./columns";

export function useContactView(view: ColumnViewName) {
    const org = useCurrentOrg();
    const user = useUser();
    const scope = React.useMemo<ViewScope>(
        () => ({ userId: user?.id ?? "", orgId: org?.id ?? "" }),
        [user?.id, org?.id],
    );
    const prefs = useViewPreferences(view, scope);
    const update = useUpdateViewPreferences(view, scope);
    const reset = useResetViewPreferences(view, scope);
    const keys = useCustomFieldKeys();

    const saved = prefs.data;
    const savedColumns = saved?.columns;
    const customKeys = React.useMemo(() => keys.data ?? [], [keys.data]);
    const { visible, available } = React.useMemo(
        () => resolveColumns(view, savedColumns, customKeys),
        [view, savedColumns, customKeys],
    );

    // Each write names only what changed, so the columns and the sort never
    // overwrite each other, whatever has or has not loaded yet.
    const { mutate } = update;
    const setColumns = React.useCallback((ids: string[]) => mutate({ columns: ids }), [mutate]);
    const setSort = React.useCallback((sort: ViewSort) => mutate({ sort }), [mutate]);
    const { mutate: resetMutate } = reset;
    const resetView = React.useCallback(() => resetMutate(), [resetMutate]);

    return {
        scope,
        columns: visible,
        available,
        customKeys,
        savedSort: saved?.sort ?? null,
        // True once the server's copy is in hand (not the browser's cached one).
        loaded: prefs.isSuccess && !prefs.isPlaceholderData,
        // A saved sort alone is a customisation too: reset is what clears it.
        customized: (savedColumns?.length ?? 0) > 0 || saved?.sort != null,
        setColumns,
        setSort,
        reset: resetView,
    };
}
