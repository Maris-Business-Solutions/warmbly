defmodule Realtime.EventBroadcasterTest do
  use ExUnit.Case, async: true

  alias Realtime.EventBroadcaster

  @removed "7f1c2a4e-0000-4000-8000-000000000001"

  defp removal do
    %{
      "event_type" => "AUDIT_CREATED",
      "entity_type" => "organization_member",
      "action" => "remove",
      "entity_id" => @removed
    }
  end

  test "a member removal names the removed user" do
    assert EventBroadcaster.removed_member(removal()) == @removed
  end

  test "other audits name nobody" do
    assert EventBroadcaster.removed_member(Map.put(removal(), "action", "update")) == nil
    assert EventBroadcaster.removed_member(Map.put(removal(), "entity_type", "team")) == nil
    assert EventBroadcaster.removed_member(Map.put(removal(), "entity_id", "")) == nil
    assert EventBroadcaster.removed_member(%{"event_type" => "EMAIL_SENT"}) == nil
  end

  test "a removal disconnects the removed user's sockets" do
    RealtimeWeb.Endpoint.subscribe("user_socket:#{@removed}")
    EventBroadcaster.broadcast(removal())

    assert_receive %Phoenix.Socket.Broadcast{
      event: "disconnect",
      topic: "user_socket:" <> @removed
    }
  end
end
