import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { listToolPolicies, revokeToolPolicy } from "@/lib/api/client/app/agent/toolPolicies";

// Workspace "always allow" policies. Refreshed by the ai_tool_policy spine entry.
export function useToolPolicies(enabled: boolean) {
    return useQuery({
        queryKey: ["ai", "tool-policies"],
        queryFn: () => listToolPolicies(),
        enabled,
        staleTime: 30_000,
    });
}

export function useRevokeToolPolicy() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (tool: string) => revokeToolPolicy(tool),
        onSuccess: () => qc.invalidateQueries({ queryKey: ["ai", "tool-policies"] }),
    });
}
