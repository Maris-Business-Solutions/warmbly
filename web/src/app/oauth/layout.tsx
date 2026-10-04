// Standalone layout for the OAuth consent screen. It requires a logged-in user
// (the human granting access) but deliberately renders WITHOUT the dashboard
// chrome — a third-party app sent the browser here, so it should look like a
// focused approval page, not the app. The route's beforeLoad sends a signed-out
// visitor to login.

import { Outlet } from "@tanstack/react-router";

import ReauthModal from "@/components/app/modals/ReauthModal";

export default function OAuthLayout() {
    return (
        <div className="min-h-screen w-full bg-slate-50 flex items-center justify-center p-4">
            <Outlet />
            {/* Approving an app needs a recent sign-in; the API client opens this prompt and retries. */}
            <ReauthModal />
        </div>
    );
}
