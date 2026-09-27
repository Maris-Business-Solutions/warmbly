defmodule Realtime.EventBroadcaster do
  @moduledoc """
  Fans a backend event out to the matching Phoenix PubSub topics: the actor's
  user channel, the org channel, and any entity channels (campaign / account /
  bulk). Routing is purely by event body fields, so the source transport (Google
  Pub/Sub via Broadway, or Redis pub/sub in dev/non-GCP envs) does not matter.
  """

  require Logger

  @doc """
  Broadcast a decoded event map. Unknown shapes are ignored.
  """
  def broadcast(event) when is_map(event) do
    user_id = event["user_id"]
    event_type = event["event_type"]

    # A removed member loses every socket at once, whatever it joined; each
    # channel's rejoin checks membership again, so only that org is lost.
    if removed = removed_member(event) do
      RealtimeWeb.Endpoint.broadcast("user_socket:#{removed}", "disconnect", %{})
    end

    if present?(user_id) do
      Phoenix.PubSub.broadcast(Realtime.PubSub, "user:#{user_id}", {:pubsub_event, event})
    end

    org_id = event["org_id"] || event["organization_id"]

    if present?(org_id) do
      # Route org events through the sequencer so each org's events are assigned a
      # monotonic seq, buffered for replay, and broadcast IN ORDER (even when
      # ingested concurrently) — the invariant resume relies on.
      Realtime.Sequencer.publish(org_id, event)
    end

    broadcast_to_entity_channels(event)

    # Mirror every event to the internal admin firehose. AdminChannel joins
    # are gated on users.admin_permissions, and with no admin connected this
    # is a broadcast to an empty topic (near-free).
    Phoenix.PubSub.broadcast(Realtime.PubSub, "admin:platform", {:pubsub_event, event})

    Logger.debug("Broadcast #{event_type}")
    :ok
  end

  def broadcast(_), do: :ok

  defp broadcast_to_entity_channels(event) do
    if campaign_id = event["campaign_id"] do
      Phoenix.PubSub.broadcast(Realtime.PubSub, "campaign:#{campaign_id}", {:pubsub_event, event})
    end

    if account_id = event["email_account_id"] do
      Phoenix.PubSub.broadcast(Realtime.PubSub, "account:#{account_id}", {:pubsub_event, event})
    end

    if operation_id = event["operation_id"] do
      Phoenix.PubSub.broadcast(Realtime.PubSub, "bulk:#{operation_id}", {:pubsub_event, event})
    end

    :ok
  end

  @doc false
  def removed_member(%{
        "event_type" => "AUDIT_CREATED",
        "entity_type" => "organization_member",
        "action" => "remove",
        "entity_id" => user_id
      })
      when is_binary(user_id) and user_id != "",
      do: user_id

  def removed_member(_event), do: nil

  defp present?(value), do: is_binary(value) and value != ""
end
