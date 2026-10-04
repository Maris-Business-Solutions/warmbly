export type APIPermissionCategory = "read" | "write" | "bulk" | "special";

export default interface APIPermission {
    name: string;
    value: number;
    description: string;
    category: APIPermissionCategory;
}

export interface APIPermissionsResponse {
    permissions: APIPermission[];
    presets: {
        read_only: number;
        full_access: number;
    };
    // What the caller may put on a key: its role's reach, or the calling key's own permissions.
    grantable: number;
    // What a third-party OAuth app may request.
    app_scopes: number;
}
