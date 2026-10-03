import RippleProvider from "@/hooks/RippleProvider";
import { ThemeProvider } from "@/components/layout/ThemeProvider";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { Outlet } from "react-router-dom";

export default function RootLayout() {
  // Keep the browser tab title in sync with the active route across the whole
  // app (auth, onboarding, dashboard). See useDocumentTitle for the route map.
  useDocumentTitle();

  return (
    <ThemeProvider>
      <RippleProvider>
        <Outlet />
      </RippleProvider>
    </ThemeProvider>
  );
}
