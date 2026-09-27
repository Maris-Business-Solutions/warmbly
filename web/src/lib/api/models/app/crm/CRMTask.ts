export type CRMTaskPriority = "low" | "medium" | "high" | "urgent";
export type CRMTaskStatus = "pending" | "in_progress" | "completed" | "cancelled";

export default interface CRMTask {
    id: string;
    organization_id: string;
    contact_id?: string;
    deal_id?: string;
    assigned_to?: string;
    assigned_team_id?: string;
    created_by: string;
    title: string;
    description?: string;
    due_date?: Date;
    priority: CRMTaskPriority;
    // The task type's name (user-managed; empty = no type).
    type: string;
    status: CRMTaskStatus;
    completed_at?: Date;
    created_at: Date;
    updated_at: Date;
}

// Create/update body: due_date goes out as the RFC3339 string the form builds.
export type CRMTaskWrite = Partial<Omit<CRMTask, "due_date">> & { due_date?: string };

export interface CRMTasksResult {
    data: CRMTask[];
    pagination: {
        has_more: boolean;
        next_cursor?: string | null;
    };
}
