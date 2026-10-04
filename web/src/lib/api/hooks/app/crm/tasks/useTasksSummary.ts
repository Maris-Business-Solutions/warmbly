import { queryOptions, useQuery } from "@tanstack/react-query";
import type SearchTasks from "@/lib/api/models/app/crm/SearchTasks";
import tasksSummary from "@/lib/api/client/app/crm/tasks/tasksSummary";

// Server-aggregated totals for the same filter the table renders. Kept as a
// separate query (not folded into the list) so the header stats stay correct
// over the whole set while the rows page in.
export const tasksSummaryQuery = (filters: SearchTasks) =>
    queryOptions({
        queryKey: ["crm", "tasks", "summary", filters],
        queryFn: () => tasksSummary(filters),
        staleTime: 30_000,
    });

export default function useTasksSummary(filters: SearchTasks, enabled = true) {
    return useQuery({
        ...tasksSummaryQuery(filters),
        enabled,
    });
}
