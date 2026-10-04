import { queryOptions, useQuery } from "@tanstack/react-query";
import getRoles from "@/lib/api/client/app/organizations/getRoles";

export const rolesQuery = queryOptions({
    queryKey: ["organizations", "roles"],
    queryFn: () => getRoles(),
});

export default function useRoles() {
    return useQuery({
        ...rolesQuery,
        select: (res) => res.data ?? [],
    })
}
