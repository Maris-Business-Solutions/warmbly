import type { CRMExternalRef } from "./CRMProvider";

export default interface ContactNote {
    id: string
    contact_id: string
    content: string
    created_by: string
    created_at: Date
    updated_at: Date
    // Set when the note lives in HubSpot.
    external?: CRMExternalRef
}
