import { useMutation, useQuery } from "@tanstack/react-query";
import type { CRMImportRequest } from "@/lib/api/models/app/crm/CRMProvider";
import listCrmLists, { type ListCrmListsParams } from "@/lib/api/client/app/crm/provider/listCrmLists";
import previewCrmImport from "@/lib/api/client/app/crm/provider/previewCrmImport";
import importCrmList from "@/lib/api/client/app/crm/provider/importCrmList";

export function useCrmLists(params: ListCrmListsParams = {}, enabled = true) {
    return useQuery({
        queryKey: ["crm", "lists", params],
        queryFn: () => listCrmLists(params),
        enabled,
        staleTime: 30_000,
    });
}

export function usePreviewCrmImport() {
    return useMutation({ mutationFn: (data: CRMImportRequest) => previewCrmImport(data) });
}

export function useImportCrmList() {
    return useMutation({ mutationFn: (data: CRMImportRequest) => importCrmList(data) });
}
