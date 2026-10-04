// Identity of the page the router is currently showing.
//
// The shell resets the content panel to the top when it changes, and must not
// when you only move around inside one page. Default identity is the full
// pathname, so every URL change is a new page. A route whose path also carries
// in-page state lists those params as stable in router.tsx:
//
//   createRoute({ path: "unibox/{-$scope}/{-$threadId}",
//     staticData: { stableParams: ["scope", "threadId"] } })
//
// Changing only a stable param keeps the page where it is, so its lists keep
// their scroll offset and its inputs keep their text (issue #396). The router
// uses the same list to decide when a page remounts.

import { useMatches } from "@tanstack/react-router";

export function useRouteKey(): string {
    return useMatches({
        select: (matches) => {
            const deepest = matches[matches.length - 1];
            if (!deepest) return "";
            // A layout that marks params stable owns the identity of every page under it.
            const owner = matches.find((m) => m.staticData?.stableParams?.length) ?? deepest;
            const stable = owner.staticData?.stableParams;
            if (!stable || stable.length === 0) return deepest.pathname;
            const params = owner.params as Record<string, string | undefined>;
            const rest = Object.keys(params)
                .filter((name) => !stable.includes(name))
                .sort()
                .map((name) => `${name}=${params[name] ?? ""}`)
                .join("&");
            return `${owner.routeId}?${rest}`;
        },
    });
}
