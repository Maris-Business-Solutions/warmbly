import { useQuery } from "@tanstack/react-query";
import getCrmSettings from "@/lib/api/client/app/crm/provider/getCrmSettings";

// The workspace's CRM mode. Every CRM surface reads it to decide whether it
// shows Warmbly's own records or HubSpot's.
export default function useCrmSettings(enabled = true) {
    return useQuery({
        queryKey: ["crm", "settings"],
        queryFn: getCrmSettings,
        staleTime: 60_000,
        enabled,
    });
}
