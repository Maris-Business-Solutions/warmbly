import { QueryClient } from "@tanstack/react-query";
import shareDeep from "./helper/shareDeep";

// Realtime events invalidate what changes, so focus never refetches; shareDeep keeps equal revived Dates identical.
export const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            staleTime: 30_000,
            gcTime: 5 * 60_000,
            refetchOnWindowFocus: false,
            refetchOnReconnect: "always",
            retry: 1,
            structuralSharing: shareDeep,
        },
        mutations: {
            retry: 0,
        },
    },
});
