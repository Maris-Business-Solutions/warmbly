// Thin wrapper around the shell. All the actual layout lives in AppShell:
// sidebar + header + content. The theme is applied app-wide in RootLayout.

import { AppShell } from "./AppShell";

export function AppLayout() {
    return <AppShell />;
}
