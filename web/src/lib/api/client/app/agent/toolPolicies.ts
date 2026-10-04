import type { AIToolPolicy } from "@/lib/api/models/app/agent/Agent";
import Request from "../../Request";

// Tools Remie runs without asking, workspace-wide. Manage settings only.
export async function listToolPolicies(): Promise<{ data: AIToolPolicy[] }> {
    return await Request<{ data: AIToolPolicy[] }>({
        method: "GET",
        url: `/ai/tool-policies`,
        authorization: true,
    });
}

export async function revokeToolPolicy(tool: string): Promise<void> {
    await Request<{ message: string }>({
        method: "DELETE",
        url: `/ai/tool-policies/${encodeURIComponent(tool)}`,
        authorization: true,
    });
}
