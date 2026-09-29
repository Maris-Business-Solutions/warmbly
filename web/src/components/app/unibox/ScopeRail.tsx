// Left-rail navigator for the unibox.
//
// Reads /unibox/overview so every count is server-truth. One visual
// language for every row: a quiet icon, the label, a bare tabular count.
//
//   Compose drafts               (only when there are any)
//   Mail: All mail / Inbox / Unread / Awaiting reply / Agent drafts / Snoozed
//         Drafts / Sent / Scheduled / Archive / Spam / Trash
//   Views                        (premade, over the automatic labels)
//   Mailboxes / Labels / Tags   (collapsible, searchable past 8 items)
//
// Every section header folds, and the fold is remembered. Mail and Views also
// have an options menu (Edit rows) to hide rows from the rail, Apple-Mail style.
// Hiding is only visual; the scope, its URL and its shortcuts keep working.
//
// Today and This week are not rows: the filter sheet's date range covers
// them, and the rail is for the places mail lives, not for every slice of it.

import React from "react";
import {
  ArchiveIcon,
  CheckIcon,
  ChevronDownIcon,
  ChevronRightIcon,
  FileTextIcon,
  InboxIcon,
  LayersIcon,
  MailIcon,
  MailOpenIcon,
  MoonIcon,
  MoreHorizontalIcon,
  OctagonAlertIcon,
  ReplyIcon,
  SearchIcon,
  SendIcon,
  SlidersHorizontalIcon,
  SparklesIcon,
  Trash2Icon,
  ClockIcon,
  FlameIcon,
  MessageSquareReplyIcon,
  BotIcon,
  BanIcon,
  TriangleAlertIcon,
} from "lucide-react";
import { useAppStore } from "@/stores";
import useUniboxOverview from "@/lib/api/hooks/app/unibox/useUniboxOverview";
import useMarkSeen from "@/lib/api/hooks/app/unibox/useMarkSeen";
import AnimatedNumber from "@/components/ui/AnimatedNumber";
import ComposeDraftsItem from "@/components/app/unibox/compose/ComposeDraftsItem";
import { cn } from "@/lib/utils";
import { DitherMeter } from "@/components/ui/dither";
import {
  PopoverMenu,
  PopoverMenuContent,
  PopoverMenuItem,
  PopoverMenuTrigger,
} from "@/components/ui/popover-menu";
import type { UniboxFolder } from "@/lib/api/models/app/unibox/UniboxSearch";
import { TagMeaningTooltip } from "@/components/ui/tag-meaning-tooltip";
import { isAutomaticTag } from "@/lib/unibox/tagMeanings";
import { UNIBOX_VIEWS, viewCategories, type UniboxViewId } from "@/lib/unibox/views";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export type UniboxScope =
  | { kind: "all" }
  | { kind: "unread" }
  | { kind: "today" }
  | { kind: "week" }
  | { kind: "awaiting" }
  | { kind: "agent_drafts" }
  | { kind: "snoozed" }
  | { kind: "scheduled" }
  | { kind: "folder"; folder: UniboxFolder }
  | { kind: "mailbox"; mailboxId: string }
  | { kind: "tag"; tagId: string }
  | { kind: "category"; categoryId: string }
  | { kind: "view"; view: UniboxViewId };

export function scopeKey(s: UniboxScope): string {
  switch (s.kind) {
    case "folder":
      return `folder:${s.folder}`;
    case "mailbox":
      return `mailbox:${s.mailboxId}`;
    case "tag":
      return `tag:${s.tagId}`;
    case "category":
      return `category:${s.categoryId}`;
    case "view":
      return `view:${s.view}`;
    default:
      return s.kind;
  }
}

const ICON = "w-[15px] h-[15px]";

const MAIL_FOLDERS: {
  folder: UniboxFolder;
  label: string;
  icon: React.ReactNode;
}[] = [
  { folder: "drafts", label: "Drafts", icon: <FileTextIcon className={ICON} /> },
  { folder: "sent", label: "Sent", icon: <SendIcon className={ICON} /> },
  { folder: "archive", label: "Archive", icon: <ArchiveIcon className={ICON} /> },
  { folder: "spam", label: "Spam", icon: <OctagonAlertIcon className={ICON} /> },
  { folder: "trash", label: "Trash", icon: <Trash2Icon className={ICON} /> },
];

const VIEW_ICONS: Record<UniboxViewId, React.ReactNode> = {
  action_required: <TriangleAlertIcon className={ICON} />,
  hot: <FlameIcon className={ICON} />,
  needs_reply: <MessageSquareReplyIcon className={ICON} />,
  follow_up: <ClockIcon className={ICON} />,
  declined: <BanIcon className={ICON} />,
  automated: <BotIcon className={ICON} />,
};

const COLLAPSE_THRESHOLD = 8;
const COLLAPSED_VISIBLE = 6;

interface ScopeRailProps {
  scope: UniboxScope;
  onChange: (s: UniboxScope) => void;
}

export function ScopeRail({ scope, onChange }: ScopeRailProps) {
  const overview = useUniboxOverview();
  const data = overview.data;
  const markSeen = useMarkSeen();

  const active = scopeKey(scope);
  const folderCounts = React.useMemo(() => {
    const m = new Map<string, { unread: number; total: number }>();
    for (const f of data?.folders ?? []) {
      m.set(f.folder, { unread: f.unread, total: f.total });
    }
    return m;
  }, [data?.folders]);

  // One descriptor per row so a section can fold, hide and edit them without
  // knowing what each one is. `key` is the scopeKey, which is also what is
  // stored when a row is hidden. `group` only decides where the small gap goes.
  const scopeRow = (
    target: UniboxScope,
    label: string,
    icon: React.ReactNode,
    count: number | undefined,
    { group = 0, accent = false }: { group?: number; accent?: boolean } = {},
  ): RailRow => {
    const key = scopeKey(target);
    return {
      key,
      label,
      icon,
      group,
      accent,
      node: (
        <Item
          icon={icon}
          label={label}
          count={count}
          accent={accent}
          active={active === key}
          onClick={() => onChange(target)}
        />
      ),
    };
  };

  const folderRow = (
    folder: UniboxFolder,
    label: string,
    icon: React.ReactNode,
    group: number,
  ): RailRow => {
    const counts = folderCounts.get(folder);
    // Drafts reads better as a total; everywhere else the number is unread.
    const count = folder === "drafts" ? counts?.total : counts?.unread;
    const accent = folder !== "drafts" && !!count;
    return {
      key: `folder:${folder}`,
      label,
      icon,
      group,
      accent,
      node: (
        <FolderItem
          icon={icon}
          label={label}
          count={count || undefined}
          accent={accent}
          active={active === `folder:${folder}`}
          onOpen={() => onChange({ kind: "folder", folder })}
          onMarkAllRead={() => markSeen.mutate({ folder, seen: true })}
        />
      ),
    };
  };

  const mailRows: RailRow[] = [
    scopeRow({ kind: "all" }, "All mail", <LayersIcon className={ICON} />, data?.total),
    folderRow("inbox", "Inbox", <InboxIcon className={ICON} />, 0),
    scopeRow({ kind: "unread" }, "Unread", <MailIcon className={ICON} />, data?.unread, {
      accent: !!data?.unread,
    }),
    scopeRow(
      { kind: "awaiting" },
      "Awaiting reply",
      <ReplyIcon className={ICON} />,
      data?.awaiting_reply,
      { accent: !!data?.awaiting_reply },
    ),
    scopeRow(
      { kind: "agent_drafts" },
      "Agent drafts",
      <SparklesIcon className={ICON} />,
      data?.awaiting_agent_draft,
      { accent: !!data?.awaiting_agent_draft },
    ),
    scopeRow({ kind: "snoozed" }, "Snoozed", <MoonIcon className={ICON} />, data?.snoozed),
    ...MAIL_FOLDERS.slice(0, 2).map((f) => folderRow(f.folder, f.label, f.icon, 1)),
    scopeRow(
      { kind: "scheduled" },
      "Scheduled",
      <ClockIcon className={ICON} />,
      data?.scheduled_pending,
      { group: 1, accent: !!data?.scheduled_pending },
    ),
    ...MAIL_FOLDERS.slice(2).map((f) => folderRow(f.folder, f.label, f.icon, 1)),
  ];

  const viewRows: RailRow[] = data
    ? UNIBOX_VIEWS.map((v) => {
        // Unread across the member labels. A thread wearing two of them
        // counts twice, which is close enough for a rail number.
        const unread = v.automated
          ? data.automated_unread
          : viewCategories(v, data.categories ?? []).reduce((n, c) => n + c.unread, 0);
        return {
          key: `view:${v.id}`,
          label: v.label,
          icon: VIEW_ICONS[v.id],
          accent: unread > 0,
          node: (
            <Tooltip>
              <TooltipTrigger asChild>
                <div>
                  <Item
                    icon={VIEW_ICONS[v.id]}
                    label={v.label}
                    hideNativeTitle
                    count={unread || undefined}
                    accent={unread > 0}
                    active={active === `view:${v.id}`}
                    onClick={() => onChange({ kind: "view", view: v.id })}
                  />
                </div>
              </TooltipTrigger>
              <TooltipContent sideOffset={6} className="max-w-72">
                {v.meaning}
              </TooltipContent>
            </Tooltip>
          ),
        };
      })
    : [];

  return (
    <nav className="h-full w-full bg-white border-r border-slate-200 overflow-y-auto py-3">
      {/* Collapses when there are no drafts. */}
      <div className="px-3 pb-2 empty:hidden">
        <ComposeDraftsItem />
      </div>

      <RailSection
        id="mail"
        label="Mail"
        rows={mailRows}
        activeKey={active}
        footer={
          // Cap meter, only once the user is materially through the allowance,
          // so the rail stays calm for the 99% case.
          data &&
          data.scheduled_pending_max > 0 &&
          data.scheduled_pending / data.scheduled_pending_max >= 0.7 && (
            <div className="px-2 pt-1.5 pb-1">
              <ScheduledMeter
                used={data.scheduled_pending}
                cap={data.scheduled_pending_max}
              />
            </div>
          )
        }
      />

      {data && data.categories && data.categories.some((c) => isAutomaticTag(c.title)) && (
        <RailSection id="views" label="Views" rows={viewRows} activeKey={active} />
      )}

      <CollapsibleSection
        id="mailboxes"
        label="Mailboxes"
        items={data?.mailboxes ?? []}
        isActive={(m) => active === `mailbox:${m.id}`}
        hasAccent={(m) => m.unread > 0}
        emptyText={overview.isPending ? "Loading…" : "No mailboxes connected."}
        searchPlaceholder="Filter mailboxes"
        getSearchKey={(m) => `${m.email} ${m.name}`}
        renderItem={(m) => (
          <Item
            key={m.id}
            icon={<MailOpenIcon className={ICON} />}
            label={m.email}
            count={m.unread || undefined}
            accent={m.unread > 0}
            active={active === `mailbox:${m.id}`}
            onClick={() => onChange({ kind: "mailbox", mailboxId: m.id })}
          />
        )}
      />

      {data && data.categories && data.categories.length > 0 && (
        <CollapsibleSection
          id="labels"
          label="Labels"
          items={data.categories}
          isActive={(c) => active === `category:${c.id}`}
          hasAccent={(c) => c.unread > 0}
          emptyText="No labels yet."
          searchPlaceholder="Filter labels"
          getSearchKey={(c) => c.title}
          renderItem={(c) => (
            <TagMeaningTooltip key={c.id} title={c.title}>
              <Item
                icon={<Dot color={c.color} />}
                label={c.title}
                hideNativeTitle={isAutomaticTag(c.title)}
                count={c.unread || c.total || undefined}
                accent={c.unread > 0}
                active={active === `category:${c.id}`}
                onClick={() => onChange({ kind: "category", categoryId: c.id })}
              />
            </TagMeaningTooltip>
          )}
        />
      )}

      {data && data.tags.length > 0 && (
        <CollapsibleSection
          id="tags"
          label="Tags"
          items={data.tags}
          isActive={(t) => active === `tag:${t.id}`}
          hasAccent={(t) => t.unread > 0}
          emptyText="No tags yet."
          searchPlaceholder="Filter tags"
          getSearchKey={(t) => t.title}
          renderItem={(t) => (
            <Item
              key={t.id}
              icon={<Dot color={t.color} />}
              label={t.title}
              count={t.unread || t.total || undefined}
              accent={t.unread > 0}
              active={active === `tag:${t.id}`}
              onClick={() => onChange({ kind: "tag", tagId: t.id })}
            />
          )}
        />
      )}
    </nav>
  );
}

function Dot({ color }: { color?: string }) {
  return (
    <span className="inline-flex w-[15px] h-[15px] items-center justify-center">
      <span
        aria-hidden
        className="block size-2 rounded-full"
        style={{ backgroundColor: color || "#94a3b8" }}
      />
    </span>
  );
}

// One rail row as a section sees it. `key` is the row's scopeKey; `node` is
// the row as it renders normally, while `icon`/`label` are all the edit mode
// needs to draw its checkbox version.
interface RailRow {
  key: string;
  label: string;
  icon: React.ReactNode;
  group?: number;
  accent?: boolean;
  node: React.ReactNode;
}

// The header every rail section shares, so the five of them look and behave
// alike: a fold toggle, an optional "unread is hiding in here" dot, and on the
// right either a count or (Mail, Views) the options menu, which becomes Done
// while rows are being edited.
function SectionHeader({
  label,
  panelId,
  open,
  onToggle,
  dot,
  count,
  editing,
  onEdit,
  optionsRef,
}: {
  label: string;
  panelId: string;
  open: boolean;
  onToggle: () => void;
  dot?: boolean;
  count?: number;
  editing?: boolean;
  onEdit?: () => void;
  optionsRef?: React.Ref<HTMLDivElement>;
}) {
  return (
    <div ref={optionsRef} className="group/section h-7 px-4 flex items-center gap-1">
      <button
        type="button"
        onClick={onToggle}
        // While rows are being edited the section is held open, so a fold
        // would change a state nobody can see until Done.
        disabled={editing}
        className="h-7 min-w-0 flex-1 flex items-center gap-1.5 text-left disabled:cursor-default"
        aria-expanded={open}
        aria-controls={panelId}
      >
        <span className="text-[10.5px] uppercase tracking-[0.12em] text-slate-400 font-medium group-hover/section:text-slate-600 transition-colors">
          {label}
        </span>
        {open ? (
          <ChevronDownIcon className="w-3 h-3 text-slate-300 group-hover/section:text-slate-500 transition-colors" />
        ) : (
          <ChevronRightIcon className="w-3 h-3 text-slate-300 group-hover/section:text-slate-500 transition-colors" />
        )}
        {/* Not only unread: Scheduled, Awaiting reply and Agent drafts
            highlight their counts too, so the dot says what it really means. */}
        {dot && (
          <>
            <span
              aria-hidden
              title="Highlighted count folded away"
              className="size-1.5 shrink-0 rounded-full bg-sky-500"
            />
            <span className="sr-only">, highlighted count folded away</span>
          </>
        )}
        {count !== undefined && (
          <span className="ml-auto text-[10.5px] text-slate-300 tabular-nums">
            {count}
          </span>
        )}
      </button>
      {onEdit &&
        (editing ? (
          <button
            type="button"
            onClick={onEdit}
            aria-label={`Done editing ${label}`}
            className="shrink-0 h-5 px-1.5 rounded text-[10.5px] font-medium text-sky-700 hover:bg-sky-50 transition-colors"
          >
            Done
          </button>
        ) : (
          // Always visible, unlike the folder menu: this is the only way in to
          // hiding rows, so it cannot wait for a hover to be found. The
          // trigger stops propagation, so opening it never also folds.
          <PopoverMenu align="end">
            <PopoverMenuTrigger asChild>
              <button
                type="button"
                data-rail-options
                aria-label={`${label} section options`}
                className="shrink-0 size-5 rounded inline-flex items-center justify-center text-slate-300 hover:text-slate-600 focus-visible:text-slate-600 hover:bg-slate-200/70 transition-colors"
              >
                <MoreHorizontalIcon className="w-3.5 h-3.5" />
              </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent>
              <PopoverMenuItem
                icon={<SlidersHorizontalIcon className="w-3 h-3" />}
                onSelect={onEdit}
              >
                Edit rows
              </PopoverMenuItem>
            </PopoverMenuContent>
          </PopoverMenu>
        ))}
    </div>
  );
}

// RailSection: a fixed list of rows (Mail, Views) that folds and that the user
// can trim. Hidden rows stay out of the rail but not out of the app: the scope,
// its URL and its shortcuts all still work, so the row you are standing on is
// never hidden from under you.
function RailSection({
  id,
  label,
  rows,
  activeKey,
  footer,
}: {
  id: string;
  label: string;
  rows: RailRow[];
  activeKey: string;
  footer?: React.ReactNode;
}) {
  const panelId = React.useId();
  const folded = useAppStore((s) => s.uniboxRailFolded[id] ?? false);
  const toggleSection = useAppStore((s) => s.toggleUniboxRailSection);
  const hiddenKeys = useAppStore((s) => s.uniboxRailHidden);
  const toggleRow = useAppStore((s) => s.toggleUniboxRailRow);
  // Local on purpose: editing is a moment, not a preference.
  const [editing, setEditing] = React.useState(false);
  // Both ends of editing swap the focused control for another one (the menu
  // trigger becomes Done, the checkboxes become rows), so focus is moved on
  // purpose: to the first checkbox going in, back to the options button out.
  // The options button is found through the header, because the menu trigger
  // puts its own ref on it.
  const headerRef = React.useRef<HTMLDivElement>(null);
  const firstEditRowRef = React.useRef<HTMLButtonElement>(null);
  const wasEditing = React.useRef(false);
  React.useEffect(() => {
    if (editing === wasEditing.current) return;
    wasEditing.current = editing;
    const target = editing
      ? firstEditRowRef.current
      : headerRef.current?.querySelector<HTMLButtonElement>("[data-rail-options]");
    target?.focus();
  }, [editing]);

  const wanted = rows.filter((r) => r.key === activeKey || !hiddenKeys.includes(r.key));
  // Folded, a section keeps only the row you are on. Editing needs every row,
  // hidden ones included, so it also unfolds the section while it lasts.
  const open = editing || !folded;
  const shown = editing ? rows : open ? wanted : wanted.filter((r) => r.key === activeKey);
  const dot = !open && wanted.some((r) => r.key !== activeKey && r.accent);

  return (
    <div
      className="pb-2"
      onKeyDown={(e) => {
        if (editing && e.key === "Escape") {
          e.stopPropagation();
          setEditing(false);
        }
      }}
    >
      <SectionHeader
        label={label}
        panelId={panelId}
        open={open}
        onToggle={() => toggleSection(id)}
        dot={dot}
        editing={editing}
        onEdit={() => setEditing((v) => !v)}
        optionsRef={headerRef}
      />
      <div id={panelId} className="px-2 space-y-px">
        {shown.map((r, i) => (
          <React.Fragment key={r.key}>
            {i > 0 && (r.group ?? 0) !== (shown[i - 1].group ?? 0) && (
              <div aria-hidden className="h-3" />
            )}
            {editing ? (
              <EditRow
                buttonRef={i === 0 ? firstEditRowRef : undefined}
                icon={r.icon}
                label={r.label}
                hidden={hiddenKeys.includes(r.key)}
                onToggle={() => toggleRow(r.key)}
              />
            ) : (
              r.node
            )}
          </React.Fragment>
        ))}
        {open && footer}
      </div>
    </div>
  );
}

function CollapsibleSection<T extends { id: string }>({
  id,
  label,
  items,
  isActive,
  hasAccent,
  emptyText,
  searchPlaceholder,
  getSearchKey,
  renderItem,
}: {
  id: string;
  label: string;
  items: T[];
  isActive: (item: T) => boolean;
  hasAccent: (item: T) => boolean;
  emptyText: string;
  searchPlaceholder: string;
  getSearchKey: (item: T) => string;
  renderItem: (item: T) => React.ReactNode;
}) {
  const panelId = React.useId();
  const [search, setSearch] = React.useState("");
  const [expanded, setExpanded] = React.useState(false);
  const sectionOpen = useAppStore((s) => !(s.uniboxRailFolded[id] ?? false));
  const toggleSection = useAppStore((s) => s.toggleUniboxRailSection);

  const filtered = React.useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((it) => getSearchKey(it).toLowerCase().includes(q));
  }, [items, search, getSearchKey]);

  const showSearch = items.length > COLLAPSE_THRESHOLD;
  const showCollapse = filtered.length > COLLAPSE_THRESHOLD;
  const visible =
    showCollapse && !expanded ? filtered.slice(0, COLLAPSED_VISIBLE) : filtered;
  const hidden = filtered.length - visible.length;
  // Folded, only the row you are on stays, even if the list would have tucked
  // it behind "Show all"; unread mail folded away leaves a dot instead.
  const foldedVisible = sectionOpen ? [] : items.filter(isActive);
  const dot = !sectionOpen && items.some((it) => !isActive(it) && hasAccent(it));

  return (
    <div className="pb-2">
      <SectionHeader
        label={label}
        panelId={panelId}
        open={sectionOpen}
        onToggle={() => toggleSection(id)}
        dot={dot}
        count={items.length}
      />

      {/* Rendered even when empty, so the header's aria-controls always has
          a target. */}
      {!sectionOpen && (
        <div id={panelId} className="px-2 space-y-px">
          {foldedVisible.map(renderItem)}
        </div>
      )}

      {sectionOpen && (
        <div id={panelId} className="px-2">
          {showSearch && (
            <div className="flex items-center gap-1.5 px-2 h-7 mb-1 rounded-md border border-slate-200 bg-white focus-within:border-sky-400 focus-within:ring-2 focus-within:ring-sky-100 transition-colors">
              <SearchIcon className="w-3 h-3 text-slate-400 shrink-0" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={searchPlaceholder}
                className="flex-1 min-w-0 h-5 bg-transparent text-[11.5px] text-slate-900 placeholder:text-slate-400 outline-none"
              />
              {search && (
                <button
                  type="button"
                  onClick={() => setSearch("")}
                  className="text-[10px] text-slate-400 hover:text-slate-600 shrink-0"
                  aria-label="Clear filter"
                >
                  clear
                </button>
              )}
            </div>
          )}

          {items.length === 0 ? (
            <div className="px-2 py-1.5 text-[11.5px] text-slate-400">
              {emptyText}
            </div>
          ) : filtered.length === 0 ? (
            <div className="px-2 py-1.5 text-[11.5px] text-slate-400">
              No matches.
            </div>
          ) : (
            <div className="space-y-px">{visible.map(renderItem)}</div>
          )}

          {hidden > 0 && (
            <button
              type="button"
              onClick={() => setExpanded(true)}
              className="w-full h-7 px-2 rounded-md text-[11.5px] text-slate-500 hover:text-slate-900 hover:bg-slate-50 transition-colors text-left"
            >
              Show all ({hidden} more)
            </button>
          )}
          {expanded && filtered.length > COLLAPSED_VISIBLE && (
            <button
              type="button"
              onClick={() => setExpanded(false)}
              className="w-full h-7 px-2 rounded-md text-[11.5px] text-slate-400 hover:text-slate-700 hover:bg-slate-50 transition-colors text-left"
            >
              Show less
            </button>
          )}
        </div>
      )}
    </div>
  );
}

// ScheduledMeter: slim usage bar for the pending-send cap. Hidden until 70%
// so it never clutters the rail; amber and rose as the cap gets close.
function ScheduledMeter({ used, cap }: { used: number; cap: number }) {
  const ratio = Math.min(1, used / cap);
  const tone = ratio >= 0.95 ? "rose" : ratio >= 0.85 ? "amber" : "sky";

  const textClasses =
    tone === "rose"
      ? "text-rose-700"
      : tone === "amber"
        ? "text-amber-700"
        : "text-slate-500";

  return (
    <div className="px-1">
      <div
        className={cn(
          "flex items-center justify-between text-[10px] mb-1",
          textClasses,
        )}
      >
        <span className="uppercase tracking-[0.12em] font-medium">Queue</span>
        <span className="tabular-nums">
          {used}/{cap}
        </span>
      </div>
      <DitherMeter frac={ratio} tone={tone} height={4} />
      {ratio >= 0.95 && (
        <p className="mt-1 text-[10px] text-rose-600 leading-snug">
          Near the limit. Cancel a few sends to free up space.
        </p>
      )}
    </div>
  );
}

const ROW =
  "w-full h-7 pl-2 pr-2 rounded-md flex items-center gap-2.5 transition-colors text-left";
const ROW_ACTIVE = "bg-slate-100 text-slate-900";
const ROW_IDLE = "text-slate-600 hover:bg-slate-50 hover:text-slate-900";

function Count({ value, accent, active }: { value: number; accent?: boolean; active?: boolean }) {
  return (
    <span
      className={cn(
        "shrink-0 tabular-nums text-[11px]",
        accent && !active
          ? "text-sky-700 font-semibold"
          : active
            ? "text-slate-600"
            : "text-slate-400",
      )}
    >
      <AnimatedNumber value={value} duration={0.4} />
    </span>
  );
}

// FolderItem: a mail folder row with a three-dot menu on the right. The row
// is a div, not a button, because the menu trigger nests inside it and nested
// buttons are invalid HTML; the trigger stops propagation so opening the menu
// never also switches folders.
function FolderItem({
  icon,
  label,
  count,
  accent,
  active,
  onOpen,
  onMarkAllRead,
}: {
  icon: React.ReactNode;
  label: string;
  count?: number;
  accent?: boolean;
  active?: boolean;
  onOpen: () => void;
  onMarkAllRead: () => void;
}) {
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onOpen}
      onKeyDown={(e) => {
        // Only the row itself activates: keydown from the nested menu trigger
        // bubbles here, and Enter on the three-dot button must open its menu.
        if (e.target !== e.currentTarget) return;
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onOpen();
        }
      }}
      aria-current={active ? "true" : undefined}
      className={cn("group/folder relative cursor-pointer pr-7 md:pr-2", ROW, active ? ROW_ACTIVE : ROW_IDLE)}
      title={label}
    >
      <span className={cn("shrink-0", active ? "text-slate-800" : "text-slate-400 group-hover/folder:text-slate-600")}>
        {icon}
      </span>
      <span className={cn("truncate min-w-0 flex-1 text-[12.5px]", active && "font-medium")}>
        {label}
      </span>
      {count !== undefined && count !== null && (
        <span className="md:group-hover/folder:invisible md:group-focus-within/folder:invisible">
          <Count value={count} accent={accent} active={active} />
        </span>
      )}
      <PopoverMenu align="end">
        {/* asChild: the trigger's own onClick already stops propagation, so
            opening the menu never also fires the row's onOpen. */}
        <PopoverMenuTrigger asChild>
          <button
            type="button"
            aria-label={`${label} folder actions`}
            className="absolute right-1 top-1 size-5 rounded inline-flex items-center justify-center text-slate-500 hover:text-slate-900 hover:bg-slate-200/70 transition-colors opacity-100 md:opacity-0 md:group-hover/folder:opacity-100 md:group-focus-within/folder:opacity-100"
          >
            <MoreHorizontalIcon className="w-3.5 h-3.5" />
          </button>
        </PopoverMenuTrigger>
        <PopoverMenuContent>
          <PopoverMenuItem
            icon={<MailOpenIcon className="w-3 h-3" />}
            onSelect={onMarkAllRead}
          >
            Mark all as read
          </PopoverMenuItem>
        </PopoverMenuContent>
      </PopoverMenu>
    </div>
  );
}

// EditRow: a row while its section is being edited. A checkbox (checked means
// shown) instead of a link, so a click changes visibility rather than scope.
function EditRow({
  buttonRef,
  icon,
  label,
  hidden,
  onToggle,
}: {
  buttonRef?: React.Ref<HTMLButtonElement>;
  icon: React.ReactNode;
  label: string;
  hidden: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      ref={buttonRef}
      type="button"
      role="checkbox"
      aria-checked={!hidden}
      onClick={onToggle}
      className={cn(ROW, ROW_IDLE, hidden && "opacity-50")}
    >
      <span
        aria-hidden
        className={cn(
          "shrink-0 size-3.5 rounded border flex items-center justify-center transition-colors",
          hidden ? "border-slate-300 bg-white" : "border-sky-600 bg-sky-600 text-white",
        )}
      >
        {!hidden && <CheckIcon className="size-2.5" strokeWidth={3} />}
      </span>
      <span className="shrink-0 text-slate-400">{icon}</span>
      <span className="truncate min-w-0 flex-1 text-[12.5px]">{label}</span>
    </button>
  );
}

function Item({
  icon,
  label,
  hideNativeTitle,
  count,
  accent,
  active,
  onClick,
}: {
  icon: React.ReactNode;
  label: string;
  hideNativeTitle?: boolean;
  count?: number;
  accent?: boolean;
  active?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      // A scope, not a page, so "true" rather than the nav's "page".
      aria-current={active ? "true" : undefined}
      className={cn("group/item", ROW, active ? ROW_ACTIVE : ROW_IDLE)}
      title={hideNativeTitle ? undefined : label}
    >
      <span className={cn("shrink-0", active ? "text-slate-800" : "text-slate-400 group-hover/item:text-slate-600")}>
        {icon}
      </span>
      <span className={cn("truncate min-w-0 flex-1 text-[12.5px]", active && "font-medium")}>
        {label}
      </span>
      {count !== undefined && count !== null && (
        <Count value={count} accent={accent} active={active} />
      )}
    </button>
  );
}
