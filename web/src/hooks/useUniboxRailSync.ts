// The scope rail's arrangement follows the member, not the browser: favorites,
// row and section order and hidden rows are saved per workspace through
// GET/PUT /me/views/unibox_rail, so a phone shows what the desktop set up. The
// persisted store stays the copy that paints the first frame. Section folds
// and pane widths are per device and never leave it.

import React from "react";
import toast from "react-hot-toast";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { getViewPreferences, updateViewPreferences } from "@/lib/api/client/app/views/views";
import type { UniboxRailLayout, ViewPreferences } from "@/lib/api/models/app/views/ViewPreferences";
import { useAppStore, useCurrentOrg, useUser, type AppStore } from "@/stores/useAppStore";
import {
  sanitizeUniboxRailFavorites,
  sanitizeUniboxRailHidden,
  sanitizeUniboxRailOrder,
} from "@/stores/slices/uiSlice";

// Long enough that a drag across several rows is one write.
const SAVE_DELAY_MS = 600;

const EMPTY: UniboxRailLayout = { favorites: [], hidden: [], order: {}, section_order: [] };

const fromStore = (s: AppStore): UniboxRailLayout => ({
  favorites: s.uniboxRailFavorites,
  hidden: s.uniboxRailHidden,
  order: s.uniboxRailOrder,
  section_order: s.uniboxRailSectionOrder,
});

const fromServer = (l: Partial<UniboxRailLayout> | null | undefined): UniboxRailLayout => ({
  favorites: sanitizeUniboxRailFavorites(l?.favorites),
  hidden: sanitizeUniboxRailHidden(l?.hidden),
  order: sanitizeUniboxRailOrder(l?.order),
  section_order: sanitizeUniboxRailHidden(l?.section_order),
});

// The same arrangement prints the same whichever order its sections were written in.
const fingerprint = (l: UniboxRailLayout): string =>
  JSON.stringify([
    l.favorites.map((f) => [f.key, f.name ?? ""]),
    l.hidden,
    Object.entries(l.order)
      .filter(([, keys]) => keys.length > 0)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)),
    l.section_order,
  ]);

const sameRail = (a: AppStore, b: AppStore) =>
  a.uniboxRailFavorites === b.uniboxRailFavorites &&
  a.uniboxRailHidden === b.uniboxRailHidden &&
  a.uniboxRailOrder === b.uniboxRailOrder &&
  a.uniboxRailSectionOrder === b.uniboxRailSectionOrder;

const apply = (l: UniboxRailLayout, owner: string) =>
  useAppStore.setState({
    uniboxRailFavorites: l.favorites,
    uniboxRailHidden: l.hidden,
    uniboxRailOrder: l.order,
    uniboxRailSectionOrder: l.section_order,
    uniboxRailOwner: owner,
  });

export function useUniboxRailSync() {
  const userId = useUser()?.id ?? "";
  const orgId = useCurrentOrg()?.id ?? "";
  const owner = userId && orgId ? `${userId}:${orgId}` : "";
  const queryClient = useQueryClient();
  const queryKey = React.useMemo(() => ["views", userId, orgId, "unibox_rail"], [userId, orgId]);
  const q = useQuery({
    queryKey,
    queryFn: async () => (await getViewPreferences("unibox_rail")).preferences ?? null,
    enabled: !!owner,
    staleTime: 30_000,
  });

  // What the account holds for this owner, as a fingerprint; null until known.
  const saved = React.useRef<string | null>(null);
  const timer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const writing = React.useRef(0);

  const save = React.useCallback(
    (layout: UniboxRailLayout) => {
      writing.current++;
      updateViewPreferences("unibox_rail", { layout })
        .then(({ preferences }) => {
          if (useAppStore.getState().uniboxRailOwner !== owner) return;
          saved.current = fingerprint(fromServer(preferences.layout));
          queryClient.setQueryData<ViewPreferences>(queryKey, preferences);
        })
        .catch(() => {
          toast.error("Couldn't save your inbox rail to your account. It's kept in this browser for now.", {
            id: "unibox-rail-save",
          });
        })
        .finally(() => {
          writing.current--;
        });
    },
    [owner, queryClient, queryKey],
  );

  // Edits go up a moment after the last one. Declared before the effect that
  // reads the server, so a new owner starts from nothing known.
  React.useEffect(() => {
    if (!owner) return;
    const unsubscribe = useAppStore.subscribe((state, prev) => {
      if (saved.current === null || sameRail(state, prev)) return;
      if (timer.current) clearTimeout(timer.current);
      timer.current = null;
      if (fingerprint(fromStore(state)) === saved.current) return;
      timer.current = setTimeout(() => {
        timer.current = null;
        const now = useAppStore.getState();
        if (now.uniboxRailOwner !== owner) useAppStore.setState({ uniboxRailOwner: owner });
        save(fromStore(now));
      }, SAVE_DELAY_MS);
    });
    return () => {
      unsubscribe();
      if (timer.current) {
        clearTimeout(timer.current);
        timer.current = null;
        // Leaving the page still saves; a workspace switch must not, because
        // the request would go out under the next workspace.
        const s = useAppStore.getState();
        if (`${s.user?.id ?? ""}:${s.currentOrganization?.id ?? ""}` === owner) save(fromStore(s));
      }
      saved.current = null;
    };
  }, [owner, save]);

  React.useEffect(() => {
    // A local edit on its way wins over what the server said before it.
    if (!owner || !q.data || timer.current || writing.current) return;
    const state = useAppStore.getState();
    if (q.data.updated_at) {
      const server = fromServer(q.data.layout);
      saved.current = fingerprint(server);
      if (state.uniboxRailOwner !== owner || fingerprint(fromStore(state)) !== saved.current) apply(server, owner);
      return;
    }
    // Nothing saved yet. A rail arranged in this browser before it was saved
    // to the account goes up once; any other member's or workspace's starts
    // from the default.
    saved.current = fingerprint(EMPTY);
    if (state.uniboxRailOwner !== null && state.uniboxRailOwner !== owner) {
      apply(EMPTY, owner);
      return;
    }
    useAppStore.setState({ uniboxRailOwner: owner });
    const local = fromStore(state);
    if (fingerprint(local) !== saved.current) save(local);
  }, [q.data, owner, save]);
}
