import RippleProvider from "@/hooks/RippleProvider";
import { ThemeProvider } from "@/components/layout/ThemeProvider";
import { DocumentTitle } from "@/hooks/useDocumentTitle";
import { Outlet } from "@tanstack/react-router";

export default function RootLayout() {
  return (
    <ThemeProvider>
      <RippleProvider>
        {/* Tab title for every page; the titles live on the routes in router.tsx. */}
        <DocumentTitle />
        <Outlet />
      </RippleProvider>
    </ThemeProvider>
  );
}
