export interface Stage {
    id: string;
    pipeline_id: string;
    name: string;
    color: string;
    position: number;
    deal_count?: number;
    created_at: Date;
    updated_at: Date;
}

export default interface Pipeline {
    id: string;
    organization_id: string;
    name: string;
    position: number;
    stages: Stage[];
    created_at: Date;
    updated_at: Date;
}
