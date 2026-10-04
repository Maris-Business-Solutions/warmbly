import type { CRMExternalRef } from "./CRMProvider";

export interface Stage {
    id: string;
    pipeline_id: string;
    name: string;
    color: string;
    position: number;
    deal_count?: number;
    created_at: Date;
    updated_at: Date;
    // HubSpot stage metadata: closed stages decide won and lost.
    closed?: boolean;
    won?: boolean;
    probability?: number;
}

export default interface Pipeline {
    id: string;
    organization_id: string;
    name: string;
    position: number;
    stages: Stage[];
    created_at: Date;
    updated_at: Date;
    // Set when the pipeline is managed in HubSpot (read-only here).
    external?: CRMExternalRef;
}
