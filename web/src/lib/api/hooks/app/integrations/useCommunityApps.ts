import { useQuery } from "@tanstack/react-query";

import getCommunityApp from "@/lib/api/client/app/integrations/getCommunityApp";
import listCommunityApps from "@/lib/api/client/app/integrations/listCommunityApps";

export function useCommunityApps() {
    return useQuery({
        queryKey: ["integrations", "community", "list"],
        queryFn: listCommunityApps,
        staleTime: 5 * 60_000,
        refetchOnWindowFocus: true,
    });
}

export function useCommunityApp(slug: string | undefined) {
    return useQuery({
        queryKey: ["integrations", "community", "app", slug],
        queryFn: () => getCommunityApp(slug as string),
        enabled: !!slug,
        retry: false,
        // Install finishes in the app's own tab; coming back should show it.
        refetchOnWindowFocus: true,
    });
}
