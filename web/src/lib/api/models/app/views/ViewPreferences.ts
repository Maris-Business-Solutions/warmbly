// A member's own layout of one dashboard list in the current workspace: the
// columns shown (in order, Name implied first) and the sort. Mirrors
// models.ViewPreferences on the backend.

// The lists that save columns and a sort.
export type ColumnViewName = "contacts" | "campaign_leads";
export type ViewName = ColumnViewName | "unibox_rail";

// The unibox scope rail's arrangement, the layout of the unibox_rail view.
// Mirrors models.UniboxRailLayout; keys are the rail's scope keys.
export interface UniboxRailLayout {
    favorites: { key: string; name?: string }[];
    hidden: string[];
    order: Record<string, string[]>;
    section_order: string[];
}

export interface ViewSort {
    by: string;
    reverse: boolean;
}

export interface ViewPreferences {
    view: ViewName;
    // Column ids in display order. Empty means the list's default layout.
    columns: string[];
    // Null or absent means the list's default sort.
    sort?: ViewSort | null;
    // Only on a view that saves a layout document (unibox_rail), once saved.
    layout?: Partial<UniboxRailLayout> | null;
    updated_at?: Date | null;
}

export interface ViewPreferencesEnvelope {
    preferences: ViewPreferences;
}

// A partial write: a field left out keeps its saved value. `columns: []` is
// the default layout; `sort: { by: "" }` the default sort.
export interface UpdateViewPreferences {
    columns?: string[];
    sort?: ViewSort;
    layout?: UniboxRailLayout | null;
}
