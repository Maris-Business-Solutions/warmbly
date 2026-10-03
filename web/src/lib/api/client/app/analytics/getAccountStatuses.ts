import type AccountStatus from "@/lib/api/models/app/analytics/AccountStatus";
import Request from "../../Request";

// Fetches the health/usage status for a specific set of mailbox ids. The page
// asks only for the mailboxes it currently shows, so the request is bounded and
// does not grow with the whole inventory.
//
// The backend wraps the list in a `{ data, pagination }` envelope and the
// shared Request helper returns the raw body. An id set this size fits one
// page, so only `data` is read here; the array fallback keeps older shapes
// working. Mirror the other list clients so callers always get a real array,
// never the envelope object (which would throw "{} is not iterable").
export default async function getAccountStatuses(
    emailIds: string[],
    signal?: AbortSignal,
): Promise<AccountStatus[]> {
    if (emailIds.length === 0) return []
    const res = await Request<{ data: AccountStatus[] | null } | AccountStatus[]>({
        method: "GET",
        url: `/analytics/accounts`,
        authorization: true,
        params: { email_ids: emailIds.join(",") },
        signal,
    })
    if (Array.isArray(res)) return res
    return res?.data ?? []
}
