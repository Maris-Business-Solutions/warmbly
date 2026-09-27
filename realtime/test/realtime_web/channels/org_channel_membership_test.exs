defmodule RealtimeWeb.OrgChannelMembershipTest do
  @moduledoc """
  Which audit events make an org channel re-read the member's permissions.
  """

  use ExUnit.Case, async: true

  alias RealtimeWeb.OrgChannel

  @me "0b7d5c1e-6a1f-4c1e-9d5a-2f0c8e4b7a10"

  defp audit(entity_type, action, entity_id \\ nil) do
    %{
      "event_type" => "AUDIT_CREATED",
      "entity_type" => entity_type,
      "action" => action,
      "entity_id" => entity_id
    }
  end

  test "a change to this member re-reads it" do
    assert OrgChannel.affects_membership?(@me, audit("organization_member", "update", @me))
  end

  test "a change to another member does not" do
    refute OrgChannel.affects_membership?(
             @me,
             audit("organization_member", "update", "someone-else")
           )
  end

  test "an updated or deleted role re-reads every member" do
    assert OrgChannel.affects_membership?(@me, audit("role", "update"))
    assert OrgChannel.affects_membership?(@me, audit("role", "delete"))
  end

  test "a new role changes nobody's permissions" do
    refute OrgChannel.affects_membership?(@me, audit("role", "create"))
  end

  test "an ownership transfer re-reads every member" do
    assert OrgChannel.affects_membership?(@me, audit("organization", "transfer"))
  end

  test "other workspace edits do not" do
    refute OrgChannel.affects_membership?(@me, audit("organization", "update"))
    refute OrgChannel.affects_membership?(@me, audit("campaign", "update"))
  end
end
