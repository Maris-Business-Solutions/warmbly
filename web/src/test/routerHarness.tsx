// Routers for tests. A suite builds the small route tree it needs, and this
// gives it the app's search handling and the app's own remount rule, so a
// page under test stays mounted, or remounts, exactly as it does in the app.

import React from "react";
import { act, render } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
    createMemoryHistory,
    createRootRoute,
    createRoute,
    createRouter,
    RouterProvider,
    type AnyRoute,
    type AnyRouter,
} from "@tanstack/react-router";
import { router as appRouter } from "@/router";
import { parseSearch, stringifySearch } from "@/lib/routerSearch";

// The router resets the window's scroll on every navigation, and jsdom only logs that it cannot.
window.scrollTo = () => {};

/** The real route's staticData (title, stableParams), so a test tree cannot drift from router.tsx. */
export function appStaticData(routeId: keyof typeof appRouter.routesById) {
    return appRouter.routesById[routeId].options.staticData ?? {};
}

/** A memory router over `routeTree`, configured like the app's. */
export function createTestRouter(routeTree: AnyRoute, initial: string): AnyRouter {
    return createRouter({
        routeTree,
        history: createMemoryHistory({ initialEntries: [initial] }),
        parseSearch,
        stringifySearch,
        // The app's rule reads stableParams off router.tsx by route id, so the
        // test tree's ids must match the real ones.
        defaultRemountDeps: appRouter.options.defaultRemountDeps,
    });
}

/** A router that matches every path and renders `children` at the root. */
export function createPassthroughRouter(children: React.ReactNode, initial = "/"): AnyRouter {
    const root = createRootRoute({ component: () => <>{children}</> });
    const any = createRoute({ getParentRoute: () => root, path: "$" });
    return createTestRouter(root.addChildren([any]), initial);
}

/** Renders `router` and waits for its first load. */
export async function renderRouter(router: AnyRouter, queryClient?: QueryClient) {
    const client = queryClient ?? new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const result = render(
        <QueryClientProvider client={client}>
            <RouterProvider router={router} />
        </QueryClientProvider>,
    );
    await act(async () => {
        await router.load();
    });
    return result;
}
