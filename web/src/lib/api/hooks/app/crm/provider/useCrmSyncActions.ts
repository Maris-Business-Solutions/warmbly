import { useMutation, useQueryClient } from "@tanstack/react-query";
import syncCrmNow from "@/lib/api/client/app/crm/provider/syncCrmNow";
import retryCrmSync from "@/lib/api/client/app/crm/provider/retryCrmSync";
import discardCrmSync from "@/lib/api/client/app/crm/provider/discardCrmSync";

export function useSyncCrmNow() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: syncCrmNow,
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["crm", "sync"] }),
    });
}

export function useRetryCrmSync() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (ids: string[] = []) => retryCrmSync(ids),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["crm", "sync"] }),
    });
}

export function useDiscardCrmSync() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (ids: string[] = []) => discardCrmSync(ids),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["crm", "sync"] }),
    });
}
