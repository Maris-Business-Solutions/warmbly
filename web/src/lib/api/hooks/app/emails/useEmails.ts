import { useEffect } from "react";
import { useInfiniteQuery, type InfiniteData } from "@tanstack/react-query";
import type GetEmails from "@/lib/api/models/app/emails/GetEmails";
import getEmails from "@/lib/api/client/app/emails/getEmails";

// Pages are fetched until the list is whole, so a larger page means fewer round trips.
const EMAILS_PAGE_LIMIT = 200;

interface UseEmailsProps {
    query: string;
    tag: string;
    limit?: number;
    enabled?: boolean;
}

// Every caller counts, selects or picks from the whole list, so this keeps paging until it is complete.
export default function useEmails({ query, tag, limit = EMAILS_PAGE_LIMIT, enabled = true }: UseEmailsProps) {
    const queryResult = useInfiniteQuery<
        GetEmails,
        Error,
        InfiniteData<GetEmails, string | null>,
        [string, string, string, string, number],
        string | null
    >({
        queryKey: ["emails", "list", query, tag, limit],
        queryFn: async ({ pageParam }) => getEmails(query, pageParam, tag, limit),
        initialPageParam: null,
        getNextPageParam: (lastPage) => {
            if (lastPage.pagination.has_more) {
                return lastPage.pagination.next_cursor
            }
            return undefined
        },
        staleTime: 5 * 60 * 1000,
        gcTime: 10 * 60 * 1000,
        enabled,
    });

    // A failed page stops the loop instead of retrying it; the caller offers the retry.
    const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = queryResult;
    useEffect(() => {
        if (enabled && hasNextPage && !isFetchingNextPage && !isFetchNextPageError) void fetchNextPage();
    }, [enabled, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage]);

    const emails = queryResult.data?.pages.flatMap((p) => p.data) ?? [];

    return {
        ...queryResult,
        emails,
        /** More pages are still on their way. */
        isLoadingRest: hasNextPage && !isFetchNextPageError,
        /** A later page failed, so `emails` is only part of the list. */
        isIncomplete: hasNextPage && isFetchNextPageError,
    };
}
