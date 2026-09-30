import type { IntegrationConnection } from "@/lib/api/models/app/integrations/Integration";
import Request from "../../Request";

export interface SetConnectionSigningKeyInput {
    connectionId: string;
    // Empty removes the key, so the inbound URL alone authenticates deliveries again.
    signing_key: string;
}

// Sets the key a Calendly or Cal.com connection's deliveries must be signed with.
export default async function setConnectionSigningKey(
    input: SetConnectionSigningKeyInput,
): Promise<{ connection: IntegrationConnection }> {
    const { connectionId, ...body } = input;
    return await Request<{ connection: IntegrationConnection }>({
        method: "PUT",
        url: `/integrations/connections/${connectionId}/signing-key`,
        data: body,
        authorization: true,
    });
}
