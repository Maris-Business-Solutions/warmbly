import { useQuery } from "@tanstack/react-query";
import getThreadLabels from "@/lib/api/client/app/unibox/getThreadLabels";

export default function useThreadLabels(threadId: string | null) {
  return useQuery({
    queryKey: ["unibox", "thread", "labels", threadId],
    queryFn: ({ signal }) => getThreadLabels(threadId!, signal),
    enabled: !!threadId,
    staleTime: 30_000,
  });
}
