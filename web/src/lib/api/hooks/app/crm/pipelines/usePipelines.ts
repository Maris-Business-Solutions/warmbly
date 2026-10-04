import { queryOptions, useQuery } from "@tanstack/react-query";
import listPipelines from "@/lib/api/client/app/crm/pipelines/listPipelines";

export const pipelinesQuery = queryOptions({
    queryKey: ["crm", "pipelines", "list"],
    queryFn: () => listPipelines(),
});

export default function usePipelines() {
    return useQuery(pipelinesQuery)
}
