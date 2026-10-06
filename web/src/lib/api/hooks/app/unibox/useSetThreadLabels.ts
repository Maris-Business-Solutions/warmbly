import { useMutation, useQueryClient, type InfiniteData } from "@tanstack/react-query";
import toast from "react-hot-toast/headless";
import setThreadLabels from "@/lib/api/client/app/unibox/setThreadLabels";
import type { UniboxListRow } from "@/lib/api/client/app/unibox/searchIncoming";
import type MiniCategory from "@/lib/api/models/app/contacts/MiniCategory";
import type User from "@/lib/api/models/auth/User";
import { errorMessage } from "@/lib/errors/message";
import { patchQueries, restoreQueries } from "@/lib/api/hooks/optimistic";

interface SearchPage {
  data: UniboxListRow[];
  pagination: { has_more: boolean; next_cursor: string | null };
}

// Lets a trigger show the save in flight from a panel it does not render.
export const setThreadLabelsKey = (threadId: string) =>
  ["unibox", "thread", "labels", "set", threadId] as const;

// Replaces a thread's conversation labels. The chips change on the click:
// the per-thread labels cache and the list rows are written from the
// workspace's categories, and put back with a toast if the server refuses.
// Once it answers, its labels prime the per-thread cache and the inbox list
// rows and the overview that powers the scope-rail category counts re-read.
export default function useSetThreadLabels(threadId: string) {
  const queryClient = useQueryClient();
  const labelsKey = ["unibox", "thread", "labels", threadId];

  return useMutation({
    mutationKey: setThreadLabelsKey(threadId),
    mutationFn: (categoryIds: string[]) =>
      setThreadLabels(threadId, categoryIds),
    onMutate: async (categoryIds) => {
      const me = queryClient.getQueryData<User>(["auth", "me"]);
      const byId = new Map((me?.categories ?? []).map((c) => [c.id, c]));
      // A label this browser has not loaded yet is left to the server's answer.
      if (categoryIds.some((id) => !byId.has(id))) return undefined;
      const labels: MiniCategory[] = categoryIds.map((id) => {
        const c = byId.get(id)!;
        return { id: c.id, title: c.title, color: c.color };
      });
      return patchQueries(
        queryClient,
        [labelsKey, ["unibox", "search"]],
        (data, key) => {
          if (key[2] === "labels" && key[3] === threadId) return labels;
          const list = data as InfiniteData<SearchPage> | undefined;
          if (!Array.isArray(list?.pages)) return data;
          return {
            ...list,
            pages: list.pages.map((page) => ({
              ...page,
              data: (page.data ?? []).map((row) =>
                (row.thread_id || row.id) === threadId ? { ...row, labels } : row,
              ),
            })),
          };
        },
      );
    },
    onError: (err, _ids, snapshot) => {
      restoreQueries(queryClient, snapshot);
      toast.error(errorMessage(err, "Couldn't update labels"));
    },
    onSuccess: (labels: MiniCategory[]) => {
      queryClient.setQueryData(labelsKey, labels);
      queryClient.invalidateQueries({ queryKey: ["unibox", "search"] });
      queryClient.invalidateQueries({ queryKey: ["unibox", "overview"] });
    },
  });
}
