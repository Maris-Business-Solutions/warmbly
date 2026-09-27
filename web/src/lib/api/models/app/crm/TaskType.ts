// A user-managed CRM task type (Call, Email, Meeting, or anything the user
// creates). Org-scoped. Tasks reference a type by its name string.
export default interface TaskType {
    id: string;
    organization_id: string;
    name: string;
    color: string;
    position: number;
    created_at: Date;
    updated_at: Date;
}
