import { useMutation, useQueryClient } from "@tanstack/react-query";
import getWebhookSecret from "@/lib/api/client/app/integrations/getWebhookSecret";
import rotateInboundUrl from "@/lib/api/client/app/integrations/rotateInboundUrl";
import setConnectionSigningKey from "@/lib/api/client/app/integrations/setConnectionSigningKey";
import testConnection from "@/lib/api/client/app/integrations/testConnection";

// Reveal (and lazily generate) the connection's outbound-webhook signing secret.
export function useRevealWebhookSecret() {
    return useMutation({ mutationFn: getWebhookSecret });
}

// Fire a synthetic event through the connection's configured automations.
export function useTestConnection() {
    return useMutation({ mutationFn: testConnection });
}

// Replace a Calendly / Cal.com connection's inbound URL.
export function useRotateInboundUrl() {
    return useMutation({ mutationFn: rotateInboundUrl });
}

// Set or remove the signing key a Calendly / Cal.com connection's deliveries must carry.
export function useSetConnectionSigningKey() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: setConnectionSigningKey,
        onSuccess: (_data, vars) => {
            qc.invalidateQueries({ queryKey: ["integrations", "connection", vars.connectionId] });
            qc.invalidateQueries({ queryKey: ["integrations", "connections"] });
        },
    });
}
