export interface AISkill {
    id: string;
    org_id: string;
    name: string;
    description: string;
    content: string;
    enabled: boolean;
    created_at: Date;
    updated_at: Date;
}

export interface CreateAISkill {
    name: string;
    description?: string;
    content?: string;
    enabled?: boolean;
}

export interface UpdateAISkill {
    name?: string;
    description?: string;
    content?: string;
    enabled?: boolean;
}
