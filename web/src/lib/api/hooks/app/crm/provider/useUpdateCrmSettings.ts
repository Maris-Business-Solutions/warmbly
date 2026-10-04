import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { UpdateCRMSettings } from "@/lib/api/models/app/crm/CRMProvider";
import updateCrmSettings from "@/lib/api/client/app/crm/provider/updateCrmSettings";

export default function useUpdateCrmSettings() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (data: UpdateCRMSettings) => updateCrmSettings(data),
        onSuccess: (settings) => {
            queryClient.setQueryData(["crm", "settings"], settings);
            // Switching the CRM swaps every pipeline, deal, task and note list.
            queryClient.invalidateQueries({ queryKey: ["crm"] });
            queryClient.invalidateQueries({ queryKey: ["contacts"] });
            queryClient.invalidateQueries({ queryKey: ["integrations"] });
        },
    });
}
