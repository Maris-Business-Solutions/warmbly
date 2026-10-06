// Shared image-upload plumbing for the campaign body editor (issue #380). The
// toolbar menu, a pasted screenshot and a dropped file all go through one path,
// so they report progress and failure identically.
//
// Uploads go to the workspace library because a body image is fetched by the
// recipient's mail client, which has no session and cannot read a presigned
// attachment URL.

import React from "react";
import toast from "react-hot-toast/headless";
import type { Editor } from "@tiptap/react";
import { useUploadEmailImage } from "@/lib/api/hooks/app/campaigns/useEmailImages";
import type EmailImage from "@/lib/api/models/app/campaigns/EmailImage";
import type { AppError } from "@/lib/api/client/normalizeError";
import buildError from "@/lib/helper/buildError";

// Only what a mail client renders. Kept in step with the upload handler's
// allowlist so the refusal happens before the request, not after it.
export const ACCEPTED_IMAGE_TYPES = "image/png,image/jpeg,image/gif,image/webp";
const ACCEPTED = new Set(["image/png", "image/jpeg", "image/jpg", "image/gif", "image/webp"]);

export function isSupportedImageFile(file: File | null | undefined): boolean {
    return !!file && ACCEPTED.has(file.type.toLowerCase());
}

// insertImage drops the image at the caret with the filename as its alt text,
// so the plain-text alternative and a client with images off both still say
// what was there.
export function insertImage(editor: Editor, image: { url: string; alt?: string; width?: number | null }) {
    editor
        .chain()
        .focus()
        .insertContent({
            type: "image",
            attrs: {
                src: image.url,
                alt: image.alt ?? "",
                width: image.width ?? null,
                align: "left",
            },
        })
        .run();
}

// useImageUpload is the one upload path: the toolbar menu, a pasted screenshot
// and a dropped file all go through it, so they report progress and failure
// identically.
export function useImageUpload() {
    const upload = useUploadEmailImage();
    const run = React.useCallback(
        async (file: File): Promise<EmailImage | null> => {
            if (!isSupportedImageFile(file)) {
                toast.error("Images must be PNG, JPG, GIF or WebP.");
                return null;
            }
            try {
                return await toast.promise(upload.mutateAsync(file), {
                    loading: `Uploading ${file.name}…`,
                    success: "Image added.",
                    error: (e: AppError) => buildError(e),
                });
            } catch {
                return null;
            }
        },
        [upload],
    );
    return { run, isUploading: upload.isPending };
}
