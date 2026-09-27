// One image in the workspace's library for email bodies. The bytes live in
// object storage under a public URL, because the recipient's mail client
// fetches them with no session of ours.
export default interface EmailImage {
    id: string;
    filename: string;
    mime_type: string;
    size: number;
    // 0 when the format carries no dimensions we can read (WebP).
    width: number;
    height: number;
    url: string;
    created_at: Date;
}

// One keyset page of the library, newest first.
export interface EmailImagePage {
    data: EmailImage[];
    pagination: {
        next_cursor?: string | null;
        has_more?: boolean;
    };
}
