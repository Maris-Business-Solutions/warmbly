// useGoogleAdminSignin: a Workspace super administrator signs in with Google to
// prove this workspace controls the domain; the callback's {state, code} finishes it.

import React from "react";
import { finishGoogleGrant, startGoogleGrant } from "@/lib/api/client/app/emails/imports/mailboxGrants";
import type { DomainGrant } from "@/lib/api/models/app/emails/MailboxSources";
import useAdminGrantPopup from "@/hooks/useAdminGrantPopup";

export default function useGoogleAdminSignin(onGranted?: (grant: DomainGrant) => void, onError?: (message: string, code?: string) => void) {
    const popup = useAdminGrantPopup({
        provider: "Google",
        windowName: "google-admin-signin",
        finish: finishGoogleGrant,
        onGranted,
        onError,
    });
    const { open } = popup;
    // Each attempt takes a fresh state: a finished or refused one cannot be reused.
    const start = React.useCallback(
        (domain: string, adminEmail: string) => open(() => startGoogleGrant({ domain, admin_email: adminEmail })),
        [open],
    );
    // Opens a sign-in the caller already started, so one start serves both proofs.
    return { busy: popup.busy, start, open, reserve: popup.reserve, reset: popup.reset };
}
