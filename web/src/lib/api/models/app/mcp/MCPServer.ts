export interface MCPTool {
    name: string;
    description: string;
    input_schema?: unknown;
}

export interface MCPServer {
    id: string;
    org_id: string;
    name: string;
    url: string;
    auth_type: "none" | "bearer";
    enabled: boolean;
    discovered_tools: MCPTool[];
    last_error?: string;
    created_at: Date;
    updated_at: Date;
}

export interface CreateMCPServer {
    name: string;
    url: string;
    auth_type: "none" | "bearer";
    token?: string;
}

export interface UpdateMCPServer {
    name?: string;
    enabled?: boolean;
    token?: string;
}
