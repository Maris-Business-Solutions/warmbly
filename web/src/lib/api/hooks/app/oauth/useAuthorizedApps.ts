import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import listAuthorizedApps from "@/lib/api/client/app/oauth/listAuthorizedApps";
import revokeAuthorizedApp from "@/lib/api/client/app/oauth/revokeAuthorizedApp";
import listWorkspaceAuthorizations from "@/lib/api/client/app/oauth/listWorkspaceAuthorizations";
import revokeWorkspaceAuthorization from "@/lib/api/client/app/oauth/revokeWorkspaceAuthorization";

export function useAuthorizedApps() {
    return useQuery({
        queryKey: ["oauth-authorized-apps", "list"],
        queryFn: () => listAuthorizedApps(),
        staleTime: 5_000,
    });
}

export function useRevokeAuthorizedApp() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (applicationId: string) => revokeAuthorizedApp(applicationId),
        onSuccess: () => void qc.invalidateQueries({ queryKey: ["oauth-authorized-apps"] }),
    });
}

// Every member's app authorizations; the workspace's credential managers see and end them.
export function useWorkspaceAuthorizations() {
    return useQuery({
        queryKey: ["oauth-authorized-apps", "workspace"],
        queryFn: () => listWorkspaceAuthorizations(),
        staleTime: 5_000,
    });
}

export function useRevokeWorkspaceAuthorization() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: ({ applicationId, userId }: { applicationId: string; userId: string }) =>
            revokeWorkspaceAuthorization(applicationId, userId),
        onSuccess: () => void qc.invalidateQueries({ queryKey: ["oauth-authorized-apps"] }),
    });
}
