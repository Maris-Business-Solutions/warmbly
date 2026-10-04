import { QueryClient } from "@tanstack/react-query";
import shareDeep from "./helper/shareDeep";

// Defaults tuned for a dashboard: lists stay fresh for half a minute (realtime
// events invalidate what changes), responses survive five minutes of
// navigation so back is instant, focus does not refetch, a dropped network
// refetches once, and one retry keeps a backend hiccup from stacking into
// seconds of waiting. shareDeep keeps equal revived Dates, so unchanged data
// keeps its identity across a refetch.
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
