import { queryOptions, useQuery } from "@tanstack/react-query";
import getAutomation from "@/lib/api/client/app/automations/getAutomation";

export const automationQuery = (id: string) =>
    queryOptions({
        queryKey: ["automations", id],
        queryFn: () => getAutomation(id),
        staleTime: 15_000,
    });

export function useAutomation(id: string, enabled = true) {
    return useQuery({
        ...automationQuery(id),
        enabled: enabled && !!id,
    });
}
