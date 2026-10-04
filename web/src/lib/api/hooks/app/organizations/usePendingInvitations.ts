import { queryOptions, useQuery } from "@tanstack/react-query";
import getPendingInvitations from "@/lib/api/client/app/organizations/getPendingInvitations";

export const pendingInvitationsQuery = queryOptions({
    queryKey: ["organizations", "invitations"],
    queryFn: () => getPendingInvitations(),
});

export default function usePendingInvitations() {
    return useQuery(pendingInvitationsQuery)
}
