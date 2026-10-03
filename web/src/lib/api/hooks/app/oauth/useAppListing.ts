import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { deleteAppListing, getAppListing, saveAppListing } from "@/lib/api/client/app/oauth/appListing";
import type { AppListingInput } from "@/lib/api/models/app/integrations/Community";

export function useAppListing(appId: string) {
    return useQuery({
        queryKey: ["oauth-app-listing", appId],
        queryFn: () => getAppListing(appId),
        staleTime: 30_000,
    });
}

export function useSaveAppListing(appId: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (input: AppListingInput) => saveAppListing(appId, input),
        onSuccess: () => {
            void qc.invalidateQueries({ queryKey: ["oauth-app-listing", appId] });
            void qc.invalidateQueries({ queryKey: ["integrations", "community"] });
        },
    });
}

export function useDeleteAppListing(appId: string) {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: () => deleteAppListing(appId),
        onSuccess: () => {
            void qc.invalidateQueries({ queryKey: ["oauth-app-listing", appId] });
            void qc.invalidateQueries({ queryKey: ["integrations", "community"] });
        },
    });
}
