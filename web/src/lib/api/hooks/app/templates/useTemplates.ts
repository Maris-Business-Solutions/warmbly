import { queryOptions, useQuery } from "@tanstack/react-query";
import listTemplates from "@/lib/api/client/app/templates/listTemplates";

export const templatesListQuery = (search?: string) =>
    queryOptions({
        queryKey: ["templates", "list", search ?? ""],
        queryFn: () => listTemplates(search),
    });

export default function useTemplates(search?: string) {
    return useQuery(templatesListQuery(search));
}
