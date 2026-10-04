import { queryOptions, useQuery } from "@tanstack/react-query";
import listTaskTypes from "@/lib/api/client/app/crm/taskTypes/listTaskTypes";

export const taskTypesQuery = queryOptions({
    queryKey: ["crm", "task-types"],
    queryFn: () => listTaskTypes(),
    staleTime: 5 * 60 * 1000,
});

export default function useTaskTypes() {
    return useQuery(taskTypesQuery);
}
