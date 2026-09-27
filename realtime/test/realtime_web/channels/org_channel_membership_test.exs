defmodule RealtimeWeb.OrgChannelMembershipTest do
  use ExUnit.Case, async: true

  alias RealtimeWeb.OrgChannel

  @removed "7f1c2a4e-0000-4000-8000-000000000001"
  @other "7f1c2a4e-0000-4000-8000-000000000002"

  defp removal(entity_id) do
    %{
      "event_type" => "AUDIT_CREATED",
      "entity_type" => "organization_member",
      "action" => "remove",
      "entity_id" => entity_id
    }
  end

  test "the removed member's channel ends" do
    assert OrgChannel.membership_ended?(removal(@removed), @removed)
  end

  test "every other member's channel stays" do
    refute OrgChannel.membership_ended?(removal(@removed), @other)
  end

  test "other member audits do not end a channel" do
    refute OrgChannel.membership_ended?(Map.put(removal(@removed), "action", "update"), @removed)

    refute OrgChannel.membership_ended?(
             Map.put(removal(@removed), "entity_type", "team"),
             @removed
           )
  end
end
