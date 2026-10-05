import { queryOptions, useQuery } from "@tanstack/react-query";
import getDashboard from "@/lib/api/client/app/analytics/getDashboard";

export const dashboardQuery = (period: string = "7d") =>
    queryOptions({
        queryKey: ["analytics", "dashboard", period],
        queryFn: () => getDashboard(period),
    });

export default function useDashboard(period: string = "7d") {
    return useQuery(dashboardQuery(period))
}
