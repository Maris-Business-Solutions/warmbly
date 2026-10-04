import { useCallback, useEffect, useState } from "react";
import {
  type CrmContext,
  type ExtensionPointApiActions,
  Alert,
  Button,
  Divider,
  Flex,
  Link,
  LoadingSpinner,
  Select,
  Tag,
  Text,
  hubspot,
} from "@hubspot/ui-extensions";

// hubspot.fetch signs every request with the app's client secret and adds
// portalId, userId and userEmail; the API acts as the matched Warmbly member.
const API = "https://api.warmbly.com/api/v1/integrations/hubspot/app";

type CardCampaign = {
  id: string;
  name: string;
  lead_status: string;
  step: string;
  held: boolean;
  hold_reason?: string;
};

type CardView = {
  connected: boolean;
  in_warmbly: boolean;
  contact_url?: string;
  status?: string;
  campaigns: CardCampaign[];
  options: { label: string; value: string }[];
};

type Props = {
  context: CrmContext;
  actions: ExtensionPointApiActions<"crm.record.sidebar">;
};

const STATUS_LABEL: Record<string, string> = {
  contacted: "Contacted",
  replied: "Replied",
  interested: "Interested",
  not_interested: "Not interested",
  meeting_booked: "Meeting booked",
  bounced: "Bounced",
  unsubscribed: "Unsubscribed",
};

const LEAD_VARIANT: Record<string, "default" | "success" | "warning" | "error"> = {
  active: "success",
  replied: "success",
  completed: "default",
  paused: "warning",
  bounced: "error",
  unsubscribed: "error",
  failed: "error",
};

hubspot.extend<"crm.record.sidebar">(({ context, actions }: Props) => (
  <WarmblyCard context={context} actions={actions} />
));

function WarmblyCard({ context, actions }: Props) {
  const contactId = String(context.crm.objectId);
  const [email, setEmail] = useState("");
  const [view, setView] = useState<CardView | null>(null);
  const [campaign, setCampaign] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const call = useCallback(
    async (path: string, body: Record<string, unknown> = {}): Promise<CardView | null> => {
      const res = await hubspot.fetch(`${API}/${path}`, {
        method: "POST",
        body: { contact_id: contactId, email, ...body },
      });
      if (res.status === 204) return null;
      const data = await res.json();
      if (!res.ok) throw new Error(data?.message || "Warmbly could not do that.");
      return data;
    },
    [contactId, email],
  );

  const load = useCallback(async () => {
    try {
      setView(await call("card"));
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [call]);

  useEffect(() => {
    actions.fetchCrmObjectProperties(["email"]).then((p) => setEmail(p.email || ""));
  }, [actions]);

  useEffect(() => {
    load();
  }, [load]);

  const act = async (path: string, body: Record<string, unknown>, done?: string) => {
    setBusy(true);
    try {
      await call(path, body);
      await load();
      if (done) actions.addAlert({ type: "success", message: done });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!view && !error) return <LoadingSpinner label="Loading Warmbly" />;
  if (!view) return <Alert title="Warmbly" variant="danger">{error}</Alert>;
  if (view && !view.connected) {
    return (
      <Text>
        This HubSpot account is not connected to Warmbly yet.{" "}
        <Link href={{ url: "https://docs.warmbly.com/guides/hubspot/", external: true }}>Set it up</Link>
      </Text>
    );
  }

  const held = view?.campaigns?.some((c) => c.held);
  return (
    <Flex direction="column" gap="sm">
      {error && <Alert title="Warmbly" variant="danger">{error}</Alert>}
      {view?.in_warmbly ? (
        <>
          <Flex direction="row" gap="xs" align="center">
            <Text format={{ fontWeight: "bold" }}>Status</Text>
            <Tag variant="default">{STATUS_LABEL[view.status] || "In Warmbly"}</Tag>
          </Flex>
          {view.campaigns.length === 0 && <Text variant="microcopy">Not in any campaign.</Text>}
          {view.campaigns.map((c) => (
            <Flex key={c.id} direction="column" gap="flush">
              <Flex direction="row" gap="xs" align="center" wrap="wrap">
                <Text format={{ fontWeight: "demibold" }}>{c.name}</Text>
                <Tag variant={c.held ? "warning" : LEAD_VARIANT[c.lead_status] || "default"}>
                  {c.held ? "Paused" : c.lead_status}
                </Tag>
              </Flex>
              <Text variant="microcopy">{c.held ? c.hold_reason || "Paused" : c.step}</Text>
            </Flex>
          ))}
          {view.campaigns.length > 0 && (
            <Button
              size="xs"
              variant="secondary"
              disabled={busy}
              onClick={() => act("pause", { paused: !held }, held ? "Campaigns resumed" : "Campaigns paused")}
            >
              {held ? "Resume campaigns" : "Pause campaigns"}
            </Button>
          )}
          <Divider />
        </>
      ) : (
        <Text variant="microcopy">Not in Warmbly yet. Add them to a campaign to start.</Text>
      )}
      {view?.options?.length > 0 ? (
        <Flex direction="column" gap="xs">
          <Select
            label="Add to campaign"
            name="campaign"
            options={view.options}
            value={campaign}
            onChange={(v) => setCampaign(String(v ?? ""))}
          />
          <Button
            size="xs"
            variant="primary"
            disabled={!campaign || busy}
            onClick={() => act("enroll", { campaign_id: campaign }, "Added to the campaign")}
          >
            Add to campaign
          </Button>
        </Flex>
      ) : (
        <Text variant="microcopy">No campaigns to add to yet.</Text>
      )}
      {view?.contact_url && (
        <Link href={{ url: view.contact_url, external: true }}>Open in Warmbly</Link>
      )}
    </Flex>
  );
}
