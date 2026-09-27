import { useEffect } from "react";

interface PagedQuery {
    hasNextPage: boolean;
    isFetchingNextPage: boolean;
    isFetchNextPageError: boolean;
    fetchNextPage: () => unknown;
}

// Keeps fetching an infinite query's pages until the list is whole. A failed page stops
// the loop instead of retrying it, so the caller can offer the retry.
export default function useAllPages(query: PagedQuery, enabled = true) {
    const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = query;
    useEffect(() => {
        if (enabled && hasNextPage && !isFetchingNextPage && !isFetchNextPageError) void fetchNextPage();
    }, [enabled, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage]);
    return {
        /** More pages are still on their way. */
        isLoadingRest: hasNextPage && !isFetchNextPageError,
        /** A later page failed, so the list is only part of the whole. */
        isIncomplete: hasNextPage && isFetchNextPageError,
    };
}
