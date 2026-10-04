import type { OAuthApplication } from "@/lib/api/models/app/oauth/OAuthApp";
import Request from "../../Request";

// The app's logo, stored and set by the server like the workspace logo.
export async function uploadOAuthApplicationLogo(id: string, blob: Blob): Promise<OAuthApplication> {
    const fd = new FormData();
    fd.append("file", blob, "logo.jpg");
    return Request<OAuthApplication>({
        method: "POST",
        url: `/oauth/applications/${id}/logo`,
        data: fd,
        authorization: true,
    });
}

export async function deleteOAuthApplicationLogo(id: string): Promise<OAuthApplication> {
    return Request<OAuthApplication>({
        method: "DELETE",
        url: `/oauth/applications/${id}/logo`,
        authorization: true,
    });
}
