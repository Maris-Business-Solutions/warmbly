import { queryOptions, useQuery } from "@tanstack/react-query";
import getUser from "../../client/auth/getUser";

// Identity rarely changes. We override the global 30s default with a
// long staleTime so navigating between pages never refetches — only
// explicit invalidations (avatar upload, onboarding completion) move
// it.
// `enabled: false` lets a public page (the /invite landing) ask only when a
// session exists, instead of firing a 401 that clears tokens.
export const userQuery = queryOptions({
    queryKey: ["auth", "me"],
    queryFn: () => getUser(),
    staleTime: 5 * 60_000,
    gcTime: 30 * 60_000,
    refetchOnMount: false,
});

export default function useUser(enabled = true) {
    return useQuery({ ...userQuery, enabled });
}
