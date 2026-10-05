import deleteCategory from "@/lib/api/client/app/categories/deleteCategory";
import type User from "@/lib/api/models/auth/User";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { patchQueries, restoreQueries } from "@/lib/api/hooks/optimistic";

// Optimistic: the label leaves the list on confirm and comes back if the server refuses.
export default function useDeleteCategory(id: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => deleteCategory(id),
    onMutate: () =>
      patchQueries(queryClient, [["auth", "me"]], (data) => {
        const user = data as User | undefined;
        if (!Array.isArray(user?.categories)) return data;
        return { ...user, categories: user.categories.filter((t) => t.id !== id) };
      }),
    onError: (_err, _vars, snapshot) => restoreQueries(queryClient, snapshot),
    onSuccess: () => {
      // The category is also a Unibox conversation label. The DB
      // cascades the unibox_thread_labels rows away, but the cached
      // list rows + scope-rail counts still reference it — refresh
      // them so deleted-label chips/counts don't linger.
      queryClient.invalidateQueries({ queryKey: ["unibox", "search"] });
      queryClient.invalidateQueries({ queryKey: ["unibox", "overview"] });
    },
  });
}
