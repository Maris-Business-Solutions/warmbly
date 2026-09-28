// Background contact imports: upload once as a draft, analyse under a
// mapping, start, then follow it. The synchronous preview/commit pair in
// importContacts.ts stays for the public API and the Sheets sync.

import Request from "../../Request";
import type { ImportCommitOptions } from "./importContacts";
import type {
    ContactImport,
    ContactImportAnalysis,
    ContactImportAnalyzeRequest,
    ContactImportList,
} from "@/lib/api/models/app/contacts/ContactImport";

export async function createContactImport(file: File, onProgress?: (fraction: number) => void): Promise<ContactImport> {
    const form = new FormData();
    form.append("file", file);
    return await Request<ContactImport>({
        method: "POST",
        url: "/contacts/imports",
        data: form,
        authorization: true,
        // A 50 MB file on a slow uplink takes a while; parsing it takes a few seconds more.
        timeout: 300_000,
        onUploadProgress: (e) => {
            if (onProgress && e.total) onProgress(e.loaded / e.total);
        },
    });
}

export async function listContactImports(limit = 10): Promise<ContactImportList> {
    return await Request<ContactImportList>({
        method: "GET",
        url: "/contacts/imports",
        params: { limit },
        authorization: true,
    });
}

export async function getContactImport(id: string): Promise<ContactImport> {
    return await Request<ContactImport>({
        method: "GET",
        url: `/contacts/imports/${id}`,
        authorization: true,
    });
}

export async function analyzeContactImport(id: string, req: ContactImportAnalyzeRequest): Promise<ContactImportAnalysis> {
    return await Request<ContactImportAnalysis>({
        method: "POST",
        url: `/contacts/imports/${id}/analyze`,
        data: req,
        authorization: true,
        timeout: 120_000,
    });
}

export async function startContactImport(id: string, opts: ImportCommitOptions): Promise<ContactImport> {
    return await Request<ContactImport>({
        method: "POST",
        url: `/contacts/imports/${id}/start`,
        data: opts,
        authorization: true,
        timeout: 60_000,
    });
}

// Stops an import; rows already written stay written.
export async function cancelContactImport(id: string): Promise<ContactImport> {
    return await Request<ContactImport>({
        method: "POST",
        url: `/contacts/imports/${id}/cancel`,
        authorization: true,
    });
}

// Every failed row as uploaded, under the file's own headers, with the reason last.
export async function downloadContactImportFailures(id: string): Promise<Blob> {
    return await Request<Blob>({
        method: "GET",
        url: `/contacts/imports/${id}/failed.csv`,
        authorization: true,
        responseType: "blob",
    });
}

// Autosaves a draft's mapping and options, so a reload resumes it.
export async function saveContactImportDraft(id: string, opts: ImportCommitOptions): Promise<ContactImport> {
    return await Request<ContactImport>({
        method: "PATCH",
        url: `/contacts/imports/${id}`,
        data: opts,
        authorization: true,
    });
}
