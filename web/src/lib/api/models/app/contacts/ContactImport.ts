import type {
    ImportColumnMapping,
    ImportCommitOptions,
    ImportPreview,
    ImportQuality,
    ImportRowError,
} from "@/lib/api/client/app/contacts/importContacts";

export type ContactImportStatus = "draft" | "queued" | "running" | "completed" | "failed" | "cancelled";

// A contact import worked off in the background: uploaded once as a draft,
// analysed, then started. CONTACT_IMPORT_PROGRESS keeps it live.
export interface ContactImport {
    id: string;
    organization_id: string;
    created_by?: string;
    filename: string;
    format: string;
    status: ContactImportStatus;
    has_header: boolean;
    columns: string[];
    /** Data rows in the file; processed is how many have settled. */
    total: number;
    processed: number;
    imported: number;
    updated: number;
    skipped: number;
    failed: number;
    options?: ImportCommitOptions;
    quality?: ImportQuality;
    segments_pinned?: boolean;
    /** Notes about the whole import rather than one row. */
    notes: string[];
    error?: string;
    created_at: Date;
    updated_at: Date;
    started_at?: Date;
    finished_at?: Date;
    /** What the mapper shows; a draft carries it so a reload resumes it. */
    preview?: ImportPreview;
    /** The first failed rows, once finished; the CSV download has them all. */
    failures?: ImportRowError[];
}

export interface ContactImportAnalysis {
    rows: number;
    new: number;
    existing: number;
    duplicates_in_file: number;
    invalid: number;
    /** Addresses the importer holds as a contact in another workspace. */
    conflicts: number;
    invalid_samples: ImportRowError[];
    quality?: ImportQuality;
    /** Why starting would be refused as a whole; absent when it would run. */
    problem?: string;
}

export interface ContactImportAnalyzeRequest {
    mapping: ImportColumnMapping[];
    has_header: boolean;
}

export interface ContactImportList {
    data: ContactImport[];
    pagination: { next_cursor: string | null; has_more: boolean };
}

export function isImportActive(status: ContactImportStatus): boolean {
    return status === "queued" || status === "running";
}

export function isImportFinished(status: ContactImportStatus): boolean {
    return status === "completed" || status === "failed" || status === "cancelled";
}
