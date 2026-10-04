import { queryOptions, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
    createForm,
    deleteForm,
    deleteFormImage,
    deleteFormSubmission,
    getForm,
    getFormsConfig,
    getFormsDomain,
    getFormStats,
    listCampaignForms,
    listForms,
    listFormSubmissions,
    setFormsDomain,
    updateForm,
    uploadFormImage,
    verifyFormsDomain,
} from "@/lib/api/client/app/forms";
import type { FormAssetKind } from "@/lib/api/client/app/forms";
import type { FormWrite } from "@/lib/api/models/app/forms/Form";
import type { FormStatsRange } from "@/lib/api/models/app/forms/FormStats";

// Every form read lives under ["forms"]: the realtime spine invalidates that
// prefix on any form mutation, and FORM_SUBMISSION_CREATED events refresh
// counters and submission lists live.
export const formsListQuery = queryOptions({ queryKey: ["forms", "list"], queryFn: listForms });

export function useForms(enabled = true) {
    return useQuery({ ...formsListQuery, enabled });
}

export const formQuery = (id: string | undefined) =>
    queryOptions({ queryKey: ["forms", id], queryFn: () => getForm(id as string) });

export function useForm(id: string | undefined) {
    return useQuery({ ...formQuery(id), enabled: !!id });
}

export function useFormsConfig() {
    return useQuery({ queryKey: ["forms", "instance-config"], queryFn: getFormsConfig, staleTime: 5 * 60 * 1000 });
}

export function useFormSubmissions(id: string | undefined, before?: string) {
    return useQuery({
        queryKey: ["forms", id, "submissions", before ?? ""],
        queryFn: () => listFormSubmissions(id as string, 50, before),
        enabled: !!id,
    });
}

function invalidateForms(queryClient: ReturnType<typeof useQueryClient>) {
    return queryClient.invalidateQueries({ queryKey: ["forms"] });
}

export function useCreateForm() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (name: string) => createForm(name),
        onSuccess: () => invalidateForms(queryClient),
    });
}

// Deliberately does NOT invalidate ["forms", id]: the builder holds the
// draft, and a refetch mid-edit would reseed an open canvas (same reasoning
// as useUpdateAutomation). The list still refreshes via the audit spine.
export function useUpdateForm() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ id, w }: { id: string; w: FormWrite }) => updateForm(id, w),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["forms", "list"] }),
    });
}

export function useDeleteForm() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (id: string) => deleteForm(id),
        onSuccess: () => invalidateForms(queryClient),
    });
}

// Lives under ["forms", id]: the audit spine and FORM_SUBMISSION events keep
// the analytics tab live without polling.
export function useFormStats(id: string | undefined, range: FormStatsRange) {
    return useQuery({
        queryKey: ["forms", id, "stats", range],
        queryFn: () => getFormStats(id as string, range),
        enabled: !!id,
    });
}

// Image uploads save immediately (they are not part of the builder draft), so
// only the list needs refreshing; the caller applies the returned form.
export function useUploadFormImage() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ id, kind, file }: { id: string; kind: FormAssetKind; file: Blob }) =>
            uploadFormImage(id, kind, file),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["forms", "list"] }),
    });
}

export function useDeleteFormImage() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ id, kind }: { id: string; kind: FormAssetKind }) => deleteFormImage(id, kind),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ["forms", "list"] }),
    });
}

export function useDeleteFormSubmission() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: ({ formId, submissionId }: { formId: string; submissionId: string }) =>
            deleteFormSubmission(formId, submissionId),
        onSuccess: () => invalidateForms(queryClient),
    });
}

// Campaign-scoped form performance. Keyed under ["forms"] so the audit spine
// and form-submission events refresh it with everything else.
export function useCampaignForms(campaignId: string | undefined) {
    return useQuery({
        queryKey: ["forms", "campaign", campaignId],
        queryFn: () => listCampaignForms(campaignId as string),
        enabled: !!campaignId,
    });
}

// The custom forms domain is workspace-wide: every form URL is built on it,
// so saving it invalidates every form read.
export function useFormsDomain() {
    return useQuery({ queryKey: ["forms", "domain"], queryFn: getFormsDomain });
}

export function useSetFormsDomain() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: (domain: string) => setFormsDomain(domain),
        onSuccess: () => invalidateForms(queryClient),
    });
}

export function useVerifyFormsDomain() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: () => verifyFormsDomain(),
        onSuccess: () => invalidateForms(queryClient),
    });
}
