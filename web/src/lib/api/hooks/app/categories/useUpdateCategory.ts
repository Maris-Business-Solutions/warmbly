import updateCategory from "@/lib/api/client/app/categories/updateCategory";
import type Tag from "@/lib/api/models/app/Tag";
import type User from "@/lib/api/models/auth/User";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { patchQueries, restoreQueries } from "@/lib/api/hooks/optimistic";

// Optimistic: a rename or recolour shows on submit; the server's copy replaces it.
export default function useUpdateCategory(id: string) {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: (tag: Partial<Tag>) => updateCategory(id, tag),
        onMutate: (tag) =>
            patchQueries(queryClient, [["auth", "me"]], (data) => {
                const user = data as User | undefined;
                if (!Array.isArray(user?.categories)) return data;
                return {
                    ...user,
                    categories: user.categories.map((c) => (c.id === id ? { ...c, ...tag, id } : c)),
                };
            }),
        onError: (_err, _tag, snapshot) => restoreQueries(queryClient, snapshot),
        onSuccess: (data) => {
            queryClient.setQueryData<User>(
                ["auth", "me"],
                (oldData) => {
                    if (!oldData) return oldData;

                    return {
                        ...oldData,
                        categories: oldData.categories.map(t => t.id === data.id ? data : t),
                    }
                }
            )
        }
    })
}
