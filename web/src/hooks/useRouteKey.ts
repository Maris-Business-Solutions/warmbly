// The page being shown: its pathname, except params a route marks `stableParams` (in-page state, issue #396).

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
