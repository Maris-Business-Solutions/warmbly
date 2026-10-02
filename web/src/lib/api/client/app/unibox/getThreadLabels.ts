// Reads the conversation labels assigned to a thread.
// GET /unibox/thread/labels?thread_id=<id>

import Request from "../../Request";
import type MiniCategory from "@/lib/api/models/app/contacts/MiniCategory";

interface Response {
  data: MiniCategory[];
}

export default async function getThreadLabels(
  threadId: string,
  signal?: AbortSignal,
): Promise<MiniCategory[]> {
  const res = await Request<Response>({
    method: "GET",
    url: `/unibox/thread/labels?thread_id=${encodeURIComponent(threadId)}`,
    authorization: true,
    signal,
  });
  return res.data ?? [];
}
