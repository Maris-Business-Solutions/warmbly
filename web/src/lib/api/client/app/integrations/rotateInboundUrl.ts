import Request from "../../Request";

// Mints a new inbound URL for a Calendly or Cal.com connection; the old one stops working at once.
export default async function rotateInboundUrl(connectionId: string): Promise<{ inbound_webhook_url: string }> {
    return await Request<{ inbound_webhook_url: string }>({
        method: "POST",
        url: `/integrations/connections/${connectionId}/rotate-inbound-url`,
        authorization: true,
    });
}
