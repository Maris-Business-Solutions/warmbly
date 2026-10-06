// Editors inside a flow's side panel report unsaved edits here, so closing the
// panel or opening another step can ask before throwing them away.

import React from "react";

type Report = (id: string, dirty: boolean) => void;

// The panel provides `report` from useDirtyRegistry.
export const DirtyContext = React.createContext<Report | null>(null);

// Outside a DirtyContext provider this does nothing, so an editor can call it anywhere.
export function useReportDirty(dirty: boolean) {
    const report = React.useContext(DirtyContext);
    const id = React.useId();
    React.useEffect(() => {
        report?.(id, dirty);
        return () => report?.(id, false);
    }, [report, id, dirty]);
}

// The panel's side: which editors hold unsaved edits right now.
export function useDirtyRegistry() {
    const dirty = React.useRef(new Set<string>());
    const report = React.useCallback<Report>((id, d) => {
        if (d) dirty.current.add(id);
        else dirty.current.delete(id);
    }, []);
    const isDirty = React.useCallback(() => dirty.current.size > 0, []);
    const clear = React.useCallback(() => dirty.current.clear(), []);
    return { report, isDirty, clear };
}
