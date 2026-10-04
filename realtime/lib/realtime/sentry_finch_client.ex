defmodule Realtime.SentryFinchClient do
  @moduledoc """
  Finch-based HTTP client for Sentry.

  Replaces the default `Sentry.HackneyClient` so the app no longer depends on
  hackney. Finch is already a transitive
  dependency via goth and broadway_cloud_pub_sub.

  Because this implements `c:Sentry.HTTPClient.child_spec/0`, Sentry starts the
  dedicated Finch pool under its own supervision tree; nothing else needs to
  start it.
  """

  @behaviour Sentry.HTTPClient

  @finch_name __MODULE__.Finch

  @impl true
  def child_spec do
    Supervisor.child_spec({Finch, name: @finch_name}, id: @finch_name)
  end

  @impl true
  def post(url, headers, body) do
    request = Finch.build(:post, url, headers, body)

    case Finch.request(request, @finch_name) do
      {:ok, %Finch.Response{status: status, headers: headers, body: body}} ->
        {:ok, status, headers, body}

      {:error, error} ->
        {:error, error}
    end
  end
end
