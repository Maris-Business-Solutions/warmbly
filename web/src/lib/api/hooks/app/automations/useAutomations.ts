import { queryOptions, useQuery } from "@tanstack/react-query";
import listAutomations from "@/lib/api/client/app/automations/listAutomations";

export const automationsQuery = queryOptions({
    queryKey: ["automations"],
    queryFn: listAutomations,
    staleTime: 15_000,
});

export function useAutomations(opts?: { enabled?: boolean }) {
    return useQuery({
        ...automationsQuery,
        enabled: opts?.enabled ?? true,
    });
}
