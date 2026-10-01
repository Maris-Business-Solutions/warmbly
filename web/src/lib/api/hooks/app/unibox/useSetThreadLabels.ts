import { useMutation, useQueryClient } from "@tanstack/react-query";
import setThreadLabels from "@/lib/api/client/app/unibox/setThreadLabels";
import type MiniCategory from "@/lib/api/models/app/contacts/MiniCategory";

// Lets a trigger show the save in flight from a panel it does not render.
export const setThreadLabelsKey = (threadId: string) =>
  ["unibox", "thread", "labels", "set", threadId] as const;

// Replaces a thread's conversation labels. On success it primes the
// per-thread labels cache and invalidates everything the label change
// touches: the (collapsed) inbox list rows that render label chips and
// the overview that powers the scope-rail category counts.
export default function useSetThreadLabels(threadId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationKey: setThreadLabelsKey(threadId),
    mutationFn: (categoryIds: string[]) =>
      setThreadLabels(threadId, categoryIds),
    onSuccess: (labels: MiniCategory[]) => {
      queryClient.setQueryData(
        ["unibox", "thread", "labels", threadId],
        labels,
      );
      queryClient.invalidateQueries({ queryKey: ["unibox", "search"] });
      queryClient.invalidateQueries({ queryKey: ["unibox", "overview"] });
    },
  });
}
