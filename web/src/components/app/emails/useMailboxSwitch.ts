import toast from "react-hot-toast/headless";
import { useQueryClient } from "@tanstack/react-query";
import { useConfirm } from "@/hooks/context/confirm";
import useUpdateEmail from "@/lib/api/hooks/app/emails/useUpdateEmail";
import buildError from "@/lib/helper/buildError";
import type { AppError } from "@/lib/api/client/normalizeError";

// Off is the whole mailbox, not campaigns alone, so the prompt points at the hold.
export function switchOffPrompt(subject: string): string {
    return `Switch ${subject} off? It stops sending, warming and syncing until you switch it back on, and keeps its settings and history. To stop campaign sending and keep warming, use Hold from campaigns in the mailbox's Overview instead.`;
}

// The mailbox's own on/off switch (status active or inactive).
export default function useMailboxSwitch(id: string, email: string) {
    const update = useUpdateEmail(id);
    const confirm = useConfirm();
    const queryClient = useQueryClient();

    const apply = async (on: boolean) => {
        try {
            await update.mutateAsync({ status: on ? "active" : "inactive" });
            void queryClient.invalidateQueries({ queryKey: ["analytics", "accounts"] });
            toast.success(on ? `${email} is back on` : `${email} switched off`);
        } catch (e) {
            toast.error(buildError(e as AppError));
        }
    };

    return {
        pending: update.isPending,
        switchOn: () => void apply(true),
        switchOff: () => confirm.show(switchOffPrompt(email), () => apply(false)),
    };
}
