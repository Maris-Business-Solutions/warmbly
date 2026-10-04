import { queryOptions, useQuery } from "@tanstack/react-query";
import getAuditLogs, { type GetAuditLogsParams } from "@/lib/api/client/app/audit/getAuditLogs";

export const auditLogsQuery = (params: GetAuditLogsParams = {}) =>
    queryOptions({
        queryKey: ["audit", "list", params],
        queryFn: () => getAuditLogs(params),
        staleTime: 30_000,
    });

export default function useAuditLogs(params: GetAuditLogsParams = {}) {
    return useQuery(auditLogsQuery(params));
}
