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
// Every section folds and can be moved; Mail and Views rows can be hidden and
// reordered. All of it is a per-browser preference: hiding is only visual, so
// a scope's URL and shortcuts keep working.
//
// Today and This week are not rows: the filter sheet's date range covers
// them, and the rail is for the places mail lives, not for every slice of it.

import React from "react";
import {
  AnimatePresence,
  LayoutGroup,
  MotionConfig,
  Reorder,
  motion,
  useDragControls,
  useReducedMotion,
  type DragControls,
} from "framer-motion";
import toast from "react-hot-toast";
import {
  ArchiveIcon,
  ArrowDownIcon,
  ArrowUpIcon,
  ChevronDownIcon,
  ChevronsDownUpIcon,
  ChevronsUpDownIcon,
  EyeIcon,
  EyeOffIcon,
  FileTextIcon,
  GripVerticalIcon,
  InboxIcon,
  LayersIcon,
  MailIcon,
  MailOpenIcon,
  MoonIcon,
  MoreHorizontalIcon,
  PencilIcon,
  OctagonAlertIcon,
  ReplyIcon,
  RotateCcwIcon,
  SearchIcon,
  SendIcon,
  SparklesIcon,
  Trash2Icon,
  ClockIcon,
  FlameIcon,
  MessageSquareReplyIcon,
  BotIcon,
  BanIcon,
  TriangleAlertIcon,
  XIcon,
} from "lucide-react";
import { useAppStore } from "@/stores";
import { applyRailOrder } from "@/stores/slices/uiSlice";
import useUniboxOverview from "@/lib/api/hooks/app/unibox/useUniboxOverview";
import useMarkSeen from "@/lib/api/hooks/app/unibox/useMarkSeen";
import AnimatedNumber from "@/components/ui/AnimatedNumber";
import ComposeDraftsItem from "@/components/app/unibox/compose/ComposeDraftsItem";
import { cn } from "@/lib/utils";
import { DitherMeter } from "@/components/ui/dither";
import { CheckSquare } from "@/components/ui/check-square";
import {
  PopoverMenu,
  PopoverMenuContent,
  PopoverMenuItem,
  PopoverMenuKbd,
  PopoverMenuSeparator,
  PopoverMenuTrigger,
} from "@/components/ui/popover-menu";
import type { UniboxFolder } from "@/lib/api/models/app/unibox/UniboxSearch";
import { TagMeaningTooltip } from "@/components/ui/tag-meaning-tooltip";
import { isAutomaticTag } from "@/lib/unibox/tagMeanings";
import { UNIBOX_VIEWS, viewCategories, type UniboxViewId } from "@/lib/unibox/views";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { shortcutLabel } from "@/components/ui/shortcut-tooltip";

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

const SECTION_IDS = ["mail", "views", "mailboxes", "labels", "tags"];

// The sidebar's fold easing, so both navigators move alike.
const EASE = [0.2, 0, 0, 1] as const;

function useRailTransition() {
  const reduce = useReducedMotion();
  return reduce ? { duration: 0 } : { duration: 0.22, ease: EASE };
}

// Each row folds its own height, so its neighbours close in instead of snapping.
const FOLD = {
  initial: { height: 0, opacity: 0, overflow: "hidden" },
  animate: { height: "auto", opacity: 1, transitionEnd: { overflow: "visible" } },
  exit: { height: 0, opacity: 0, overflow: "hidden" },
} as const;

const sameOrder = (a: string[], b: string[]) =>
  a.length === b.length && a.every((k, i) => k === b[i]);

// Puts `key` just before or after `neighbour`, the rest keeping their order.
function moveNextTo(order: string[], key: string, neighbour: string, delta: -1 | 1) {
  const out = order.filter((k) => k !== key);
  const n = out.indexOf(neighbour);
  out.splice(delta < 0 ? n : n + 1, 0, key);
  return out;
}

interface ScopeRailProps {
  scope: UniboxScope;
  onChange: (s: UniboxScope) => void;
}

// What a section needs to fold its siblings and move itself among them.
interface SectionControls {
  isFirst: boolean;
  isLast: boolean;
  move: (delta: -1 | 1) => void;
  canFoldOthers: boolean;
  anyFolded: boolean;
  foldOthers: () => void;
  unfoldAll: () => void;
}

export function ScopeRail({ scope, onChange }: ScopeRailProps) {
  const overview = useUniboxOverview();
  const data = overview.data;
  const markSeen = useMarkSeen();
  // The page and the mobile sheet both mount a rail; each slides its own highlight.
  const layoutGroup = React.useId();
  const transition = useRailTransition();
  const sectionOrder = useAppStore((s) => s.uniboxRailSectionOrder);
  const setSectionOrder = useAppStore((s) => s.setUniboxRailSectionOrder);
  const folded = useAppStore((s) => s.uniboxRailFolded);
  const setFolded = useAppStore((s) => s.setUniboxRailFolded);

  const active = scopeKey(scope);
  const folderCounts = React.useMemo(() => {
    const m = new Map<string, { unread: number; total: number }>();
    for (const f of data?.folders ?? []) {
      m.set(f.folder, { unread: f.unread, total: f.total });
    }
    return m;
  }, [data?.folders]);

  const scopeRow = (
    target: UniboxScope,
    label: string,
    icon: React.ReactNode,
    count: number | undefined,
    { accent = false }: { accent?: boolean } = {},
  ): RailRow => ({ key: scopeKey(target), label, icon, count, accent, onOpen: () => onChange(target) });

  const folderRow = (folder: UniboxFolder, label: string, icon: React.ReactNode): RailRow => {
    const counts = folderCounts.get(folder);
    // Drafts reads better as a total; everywhere else the number is unread.
    const count = folder === "drafts" ? counts?.total : counts?.unread;
    return {
      key: `folder:${folder}`,
      label,
      icon,
      count: count || undefined,
      accent: folder !== "drafts" && !!count,
      noun: "folder",
      onOpen: () => onChange({ kind: "folder", folder }),
      onMarkAllRead: () => markSeen.mutate({ folder, seen: true }),
    };
  };

  const mailRows: RailRow[] = [
    scopeRow({ kind: "all" }, "All mail", <LayersIcon className={ICON} />, data?.total),
    folderRow("inbox", "Inbox", <InboxIcon className={ICON} />),
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
    ...MAIL_FOLDERS.slice(0, 2).map((f) => folderRow(f.folder, f.label, f.icon)),
    scopeRow(
      { kind: "scheduled" },
      "Scheduled",
      <ClockIcon className={ICON} />,
      data?.scheduled_pending,
      { accent: !!data?.scheduled_pending },
    ),
    ...MAIL_FOLDERS.slice(2).map((f) => folderRow(f.folder, f.label, f.icon)),
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
          count: unread || undefined,
          accent: unread > 0,
          noun: "view",
          tooltip: v.meaning,
          onOpen: () => onChange({ kind: "view", view: v.id }),
        };
      })
    : [];

  const present = applyRailOrder(SECTION_IDS, sectionOrder).filter((id) => {
    if (id === "views") return !!data?.categories?.some((c) => isAutomaticTag(c.title));
    if (id === "labels") return !!data?.categories?.length;
    if (id === "tags") return !!data?.tags?.length;
    return true;
  });

  const controls = (id: string): SectionControls => {
    const at = present.indexOf(id);
    return {
      isFirst: at <= 0,
      isLast: at === present.length - 1,
      move: (delta) => {
        const neighbour = present[at + delta];
        if (!neighbour) return;
        const next = moveNextTo(applyRailOrder(SECTION_IDS, sectionOrder), id, neighbour, delta);
        setSectionOrder(sameOrder(next, SECTION_IDS) ? [] : next);
      },
      canFoldOthers: present.some((s) => s !== id && !folded[s]),
      anyFolded: present.some((s) => folded[s]),
      foldOthers: () => setFolded(Object.fromEntries(present.map((s) => [s, s !== id]))),
      unfoldAll: () => setFolded(Object.fromEntries(present.map((s) => [s, false]))),
    };
  };

  const section = (id: string) => {
    switch (id) {
      case "mail":
        return (
          <RailSection
            id="mail"
            label="Mail"
            rows={mailRows}
            activeKey={active}
            controls={controls("mail")}
            footer={
              // The queue meter only once the allowance is 70% used.
              data &&
              data.scheduled_pending_max > 0 &&
              data.scheduled_pending / data.scheduled_pending_max >= 0.7 && (
                <div className="px-2 pt-1.5 pb-1">
                  <ScheduledMeter used={data.scheduled_pending} cap={data.scheduled_pending_max} />
                </div>
              )
            }
          />
        );
      case "views":
        return (
          <RailSection id="views" label="Views" rows={viewRows} activeKey={active} controls={controls("views")} />
        );
      case "mailboxes":
        return (
          <CollapsibleSection
            id="mailboxes"
            label="Mailboxes"
            controls={controls("mailboxes")}
            items={data?.mailboxes ?? []}
            isActive={(m) => active === `mailbox:${m.id}`}
            hasAccent={(m) => m.unread > 0}
            emptyText={overview.isPending ? "Loading…" : "No mailboxes connected."}
            searchPlaceholder="Filter mailboxes"
            getSearchKey={(m) => `${m.email} ${m.name}`}
            renderItem={(m) => (
              <Item
                icon={<MailOpenIcon className={ICON} />}
                label={m.email}
                count={m.unread || undefined}
                accent={m.unread > 0}
                active={active === `mailbox:${m.id}`}
                onClick={() => onChange({ kind: "mailbox", mailboxId: m.id })}
              />
            )}
          />
        );
      case "labels":
        return (
          <CollapsibleSection
            id="labels"
            label="Labels"
            controls={controls("labels")}
            items={data?.categories ?? []}
            isActive={(c) => active === `category:${c.id}`}
            hasAccent={(c) => c.unread > 0}
            emptyText="No labels yet."
            searchPlaceholder="Filter labels"
            getSearchKey={(c) => c.title}
            renderItem={(c) => (
              <TagMeaningTooltip title={c.title}>
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
        );
      case "tags":
        return (
          <CollapsibleSection
            id="tags"
            label="Tags"
            controls={controls("tags")}
            items={data?.tags ?? []}
            isActive={(t) => active === `tag:${t.id}`}
            hasAccent={(t) => t.unread > 0}
            emptyText="No tags yet."
            searchPlaceholder="Filter tags"
            getSearchKey={(t) => t.title}
            renderItem={(t) => (
              <Item
                icon={<Dot color={t.color} />}
                label={t.title}
                count={t.unread || t.total || undefined}
                accent={t.unread > 0}
                active={active === `tag:${t.id}`}
                onClick={() => onChange({ kind: "tag", tagId: t.id })}
              />
            )}
          />
        );
      default:
        return null;
    }
  };

  // Arrow keys walk the rows like a list; Tab still visits every control.
  const onRailKeyDown = (e: React.KeyboardEvent<HTMLElement>) => {
    if (e.altKey || e.metaKey || e.ctrlKey || e.shiftKey) return;
    if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) return;
    const target = e.target as HTMLElement;
    if (!target.matches("[data-rail-row]")) return;
    const rows = Array.from(e.currentTarget.querySelectorAll<HTMLElement>("[data-rail-row]"));
    const at = rows.indexOf(target);
    const next =
      e.key === "Home" ? 0 : e.key === "End" ? rows.length - 1 : at + (e.key === "ArrowDown" ? 1 : -1);
    if (next < 0 || next >= rows.length) return;
    e.preventDefault();
    e.stopPropagation();
    rows[next].focus();
  };

  return (
    <MotionConfig reducedMotion="user">
      <LayoutGroup id={layoutGroup}>
        <motion.nav
          layoutScroll
          onKeyDown={onRailKeyDown}
          className="h-full w-full bg-white border-r border-slate-200 overflow-y-auto py-3"
        >
          {/* Collapses when there are no drafts. */}
          <div className="px-3 pb-2 empty:hidden">
            <ComposeDraftsItem />
          </div>
          {present.map((id) => (
            <motion.div key={id} layout="position" transition={transition}>
              {section(id)}
            </motion.div>
          ))}
        </motion.nav>
      </LayoutGroup>
    </MotionConfig>
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

// One Mail or Views row. `key` is its scopeKey, which is what hiding and
// ordering store.
interface RailRow {
  key: string;
  label: string;
  icon: React.ReactNode;
  count?: number;
  accent?: boolean;
  noun?: "folder" | "view";
  tooltip?: string;
  onOpen: () => void;
  onMarkAllRead?: () => void;
}

// A menu that opens from its "…" button or, on right-click, at the pointer.
function useAnchoredMenu() {
  const [open, setOpen] = React.useState(false);
  const [point, setPoint] = React.useState<{ x: number; y: number } | null>(null);
  const onContextMenu = (e: React.MouseEvent<HTMLElement>) => {
    e.preventDefault();
    e.stopPropagation();
    // A keyboard context-menu key reports 0,0: open under the element instead.
    const r = e.currentTarget.getBoundingClientRect();
    setPoint(e.clientX || e.clientY ? { x: e.clientX, y: e.clientY } : { x: r.left + 12, y: r.bottom });
    setOpen(true);
  };
  const menuProps = {
    open,
    anchorPoint: point,
    onOpenChange: (o: boolean) => {
      if (o) setPoint(null);
      setOpen(o);
    },
  };
  return { open, onContextMenu, menuProps };
}

// The header every rail section shares: a fold toggle, a dot when a folded
// section hides a highlighted count, and on the right a count or the row
// editor's pencil, then the section's own menu.
function SectionHeader({
  label,
  panelId,
  open,
  onToggle,
  dot,
  count,
  editing,
  onEdit,
  hiddenCount,
  menu,
}: {
  label: string;
  panelId: string;
  open: boolean;
  onToggle: () => void;
  dot?: boolean;
  count?: number;
  editing?: boolean;
  onEdit?: () => void;
  hiddenCount?: number;
  menu: React.ReactNode;
}) {
  const hiddenId = React.useId();
  const transition = useRailTransition();
  const sectionMenu = useAnchoredMenu();
  const pop = {
    initial: { opacity: 0, scale: 0.6 },
    animate: { opacity: 1, scale: 1 },
    exit: { opacity: 0, scale: 0.6 },
    transition,
  };
  return (
    <div
      className="group/section h-7 px-4 flex items-center gap-1"
      onContextMenu={editing ? undefined : sectionMenu.onContextMenu}
    >
      <button
        type="button"
        onClick={onToggle}
        // Held open while rows are edited, so Done never folds by surprise.
        disabled={editing}
        className="h-7 min-w-0 flex-1 flex items-center gap-1.5 text-left rounded disabled:cursor-default focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
        aria-expanded={open}
        aria-controls={panelId}
      >
        <span className="text-[10.5px] uppercase tracking-[0.12em] text-slate-400 font-medium group-hover/section:text-slate-600 transition-colors">
          {label}
        </span>
        <ChevronDownIcon
          aria-hidden
          className={cn(
            "w-3 h-3 shrink-0 text-slate-300 group-hover/section:text-slate-500 transition-[transform,color] duration-200 ease-out motion-reduce:transition-none",
            !open && "-rotate-90",
          )}
        />
        {/* "Highlighted", not "unread": Scheduled and Awaiting reply raise it too. */}
        <AnimatePresence initial={false}>
          {dot && (
            <motion.span key="dot" className="flex" {...pop}>
              <span
                aria-hidden
                title="Highlighted count folded away"
                className="size-1.5 shrink-0 rounded-full bg-sky-500"
              />
              <span className="sr-only">, highlighted count folded away</span>
            </motion.span>
          )}
        </AnimatePresence>
        {count !== undefined && (
          <span className="ml-auto text-[10.5px] text-slate-300 tabular-nums">{count}</span>
        )}
      </button>
      {/* Says rows are off the rail, and describes the pencil to a screen reader. */}
      <AnimatePresence initial={false}>
        {!!hiddenCount && (
          <motion.span
            key="hidden"
            id={hiddenId}
            className="shrink-0 text-[10.5px] text-slate-500 tabular-nums"
            {...pop}
          >
            {hiddenCount} hidden
          </motion.span>
        )}
      </AnimatePresence>
      {onEdit && (
        <AnimatePresence initial={false} mode="popLayout">
          {editing ? (
            <motion.button
              key="done"
              type="button"
              onClick={onEdit}
              aria-label={`Done editing ${label}`}
              className="shrink-0 h-5 px-1.5 rounded text-[10.5px] font-medium text-sky-700 hover:bg-sky-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 transition-colors"
              {...pop}
            >
              Done
            </motion.button>
          ) : (
            // Always visible: it is the way in to hiding and ordering rows.
            <motion.button
              key="edit"
              type="button"
              data-rail-options
              onClick={onEdit}
              title={`Edit ${label} rows`}
              aria-label={`Edit ${label} rows`}
              aria-describedby={hiddenCount ? hiddenId : undefined}
              className="shrink-0 size-5 rounded inline-flex items-center justify-center text-slate-400 hover:text-slate-700 focus-visible:text-slate-700 hover:bg-slate-200/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 transition-colors"
              {...pop}
            >
              <PencilIcon className="w-3.5 h-3.5" />
            </motion.button>
          )}
        </AnimatePresence>
      )}
      {!editing && (
        <PopoverMenu align="end" {...sectionMenu.menuProps}>
          <PopoverMenuTrigger asChild>
            <button
              type="button"
              aria-label={`${label} section options`}
              title={`${label} options`}
              className="shrink-0 size-5 rounded inline-flex items-center justify-center text-slate-400 hover:text-slate-700 hover:bg-slate-200/70 focus-visible:text-slate-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 transition-[opacity,color,background-color] opacity-100 md:opacity-0 md:group-hover/section:opacity-100 md:group-focus-within/section:opacity-100 data-[state=open]:opacity-100 data-[state=open]:bg-slate-200/70 data-[state=open]:text-slate-700"
            >
              <MoreHorizontalIcon className="w-3.5 h-3.5" />
            </button>
          </PopoverMenuTrigger>
          <PopoverMenuContent minWidth={210}>{menu}</PopoverMenuContent>
        </PopoverMenu>
      )}
    </div>
  );
}

const MENU_ICON = "w-3 h-3";

// What every section's menu starts with: folding and moving the section.
function SectionMenuItems({
  open,
  onToggle,
  controls,
  children,
}: {
  open: boolean;
  onToggle: () => void;
  controls: SectionControls;
  children?: React.ReactNode;
}) {
  return (
    <>
      <PopoverMenuItem
        icon={open ? <ChevronsDownUpIcon className={MENU_ICON} /> : <ChevronsUpDownIcon className={MENU_ICON} />}
        onSelect={onToggle}
      >
        {open ? "Fold section" : "Unfold section"}
      </PopoverMenuItem>
      <PopoverMenuItem
        icon={<ChevronsDownUpIcon className={MENU_ICON} />}
        disabled={!controls.canFoldOthers}
        onSelect={controls.foldOthers}
      >
        Fold other sections
      </PopoverMenuItem>
      <PopoverMenuItem
        icon={<ChevronsUpDownIcon className={MENU_ICON} />}
        disabled={!controls.anyFolded}
        onSelect={controls.unfoldAll}
      >
        Unfold all sections
      </PopoverMenuItem>
      <PopoverMenuSeparator />
      <PopoverMenuItem
        icon={<ArrowUpIcon className={MENU_ICON} />}
        disabled={controls.isFirst}
        onSelect={() => controls.move(-1)}
      >
        Move section up
      </PopoverMenuItem>
      <PopoverMenuItem
        icon={<ArrowDownIcon className={MENU_ICON} />}
        disabled={controls.isLast}
        onSelect={() => controls.move(1)}
      >
        Move section down
      </PopoverMenuItem>
      {children && (
        <>
          <PopoverMenuSeparator />
          {children}
        </>
      )}
    </>
  );
}

// RailSection: a fixed list of rows (Mail, Views) that folds, and whose rows
// the user can hide and reorder. The row you are on is never hidden.
function RailSection({
  id,
  label,
  rows,
  activeKey,
  controls,
  footer,
}: {
  id: string;
  label: string;
  rows: RailRow[];
  activeKey: string;
  controls: SectionControls;
  footer?: React.ReactNode;
}) {
  const panelId = React.useId();
  const transition = useRailTransition();
  const folded = useAppStore((s) => s.uniboxRailFolded[id] ?? false);
  const toggleSection = useAppStore((s) => s.toggleUniboxRailSection);
  const hiddenKeys = useAppStore((s) => s.uniboxRailHidden);
  const toggleRow = useAppStore((s) => s.toggleUniboxRailRow);
  const setRowsHidden = useAppStore((s) => s.setUniboxRailRowsHidden);
  const storedOrder = useAppStore((s) => s.uniboxRailOrder[id]);
  const setOrder = useAppStore((s) => s.setUniboxRailOrder);
  // Local on purpose: editing is a moment, not a preference.
  const [editing, setEditing] = React.useState(false);
  // The order while a drag is in flight; committed once, on the drop.
  const [draft, setDraft] = React.useState<string[] | null>(null);
  // Mirrors `draft` so a drop that lands before the next render still commits it.
  const draftRef = React.useRef<string[] | null>(null);
  const updateDraft = (keys: string[] | null) => {
    draftRef.current = keys;
    setDraft(keys);
  };

  const [announcement, setAnnouncement] = React.useState("");
  // Rows cross-fade between modes only after a toggle, never on first paint.
  const [swapped, setSwapped] = React.useState(false);
  const sectionRef = React.useRef<HTMLDivElement>(null);
  const firstEditRowRef = React.useRef<HTMLButtonElement>(null);
  const wasEditing = React.useRef(false);

  const setEditMode = (on: boolean) => {
    setSwapped(true);
    updateDraft(null);
    setEditing(on);
  };

  // Focus follows the swap: into the checkboxes, then back to the pencil,
  // unless the user has already put it somewhere else.
  React.useEffect(() => {
    if (editing === wasEditing.current) return;
    wasEditing.current = editing;
    if (editing) {
      firstEditRowRef.current?.focus();
      return;
    }
    const focused = document.activeElement;
    if (focused && focused !== document.body && !sectionRef.current?.contains(focused)) return;
    sectionRef.current?.querySelector<HTMLButtonElement>("[data-rail-options]")?.focus();
  }, [editing]);

  // A click anywhere else ends editing, as Done would.
  React.useEffect(() => {
    if (!editing) return;
    const onDown = (e: MouseEvent) => {
      const t = e.target as Element | null;
      if (!t || sectionRef.current?.contains(t) || t.closest?.("[data-floating]")) return;
      setEditing(false);
      draftRef.current = null;
      setDraft(null);
    };
    document.addEventListener("mousedown", onDown, true);
    return () => document.removeEventListener("mousedown", onDown, true);
  }, [editing]);

  const defaults = rows.map((r) => r.key);
  const orderKeys = draft ?? applyRailOrder(defaults, storedOrder);
  const byKey = new Map(rows.map((r) => [r.key, r]));
  const ordered = orderKeys.flatMap((k) => byKey.get(k) ?? []);
  const isHidden = (k: string) => hiddenKeys.includes(k);
  const wanted = ordered.filter((r) => r.key === activeKey || !isHidden(r.key));
  const open = editing || !folded;
  const shown = editing ? ordered : open ? wanted : wanted.filter((r) => r.key === activeKey);
  const dot = !open && wanted.some((r) => r.key !== activeKey && r.accent);
  const hiddenHere = defaults.filter(isHidden);
  const isDefault = hiddenHere.length === 0 && sameOrder(applyRailOrder(defaults, storedOrder), defaults);

  const commit = (keys: string[]) => setOrder(id, sameOrder(keys, defaults) ? null : keys);

  // Steps past the neighbour the user can see, so a hidden row never swallows a
  // move; a folded section shows no neighbours, so nothing moves there.
  const visibleForMove = editing ? ordered : open ? wanted : [];
  const canMove = (key: string, delta: -1 | 1) => {
    const at = visibleForMove.findIndex((r) => r.key === key);
    return at >= 0 && !!visibleForMove[at + delta];
  };
  const move = (key: string, delta: -1 | 1) => {
    const at = visibleForMove.findIndex((r) => r.key === key);
    const neighbour = visibleForMove[at + delta];
    if (at < 0 || !neighbour) return;
    commit(moveNextTo(orderKeys, key, neighbour.key, delta));
    setAnnouncement(`${byKey.get(key)?.label} moved to position ${at + delta + 1} of ${visibleForMove.length}`);
  };

  const hideWithUndo = (row: RailRow) => {
    setRowsHidden([row.key], true);
    const when = row.key === activeKey ? " once you leave it" : "";
    toast(
      (t) => (
        <span className="flex items-center gap-3">
          <span>
            {row.label} is hidden from the rail{when}.
          </span>
          <button
            type="button"
            onClick={() => {
              setRowsHidden([row.key], false);
              toast.dismiss(t.id);
            }}
            className="shrink-0 font-medium text-sky-700 hover:text-sky-800"
          >
            Undo
          </button>
        </span>
      ),
      { id: `rail-hidden:${row.key}` },
    );
  };

  const reset = () => {
    setRowsHidden(defaults, false);
    setOrder(id, null);
    updateDraft(null);
    setAnnouncement(`${label} rows are back to the default`);
  };

  const menu = (
    <SectionMenuItems open={open} onToggle={() => toggleSection(id)} controls={controls}>
      <PopoverMenuItem icon={<PencilIcon className={MENU_ICON} />} onSelect={() => setEditMode(true)}>
        Edit rows…
      </PopoverMenuItem>
      {hiddenHere.length > 0 && (
        <PopoverMenuItem icon={<EyeIcon className={MENU_ICON} />} onSelect={() => setRowsHidden(hiddenHere, false)}>
          Show {hiddenHere.length} hidden {hiddenHere.length === 1 ? "row" : "rows"}
        </PopoverMenuItem>
      )}
      <PopoverMenuItem icon={<RotateCcwIcon className={MENU_ICON} />} disabled={isDefault} onSelect={reset}>
        Reset to default
      </PopoverMenuItem>
    </SectionMenuItems>
  );

  return (
    <div
      ref={sectionRef}
      className="pb-2"
      onKeyDown={(e) => {
        if (editing && e.key === "Escape") {
          e.stopPropagation();
          setEditMode(false);
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
        onEdit={() => setEditMode(!editing)}
        hiddenCount={editing ? 0 : hiddenHere.length}
        menu={menu}
      />
      <Reorder.Group
        as="div"
        axis="y"
        id={panelId}
        values={shown.map((r) => r.key)}
        onReorder={(keys: string[]) => editing && updateDraft(keys)}
        className="px-2 space-y-px"
      >
        <AnimatePresence initial={false}>
          {shown.map((r, i) => (
            <RailRowItem
              key={r.key}
              row={r}
              sectionLabel={label}
              active={r.key === activeKey}
              editing={editing}
              swapped={swapped}
              hidden={isHidden(r.key)}
              buttonRef={i === 0 ? firstEditRowRef : undefined}
              canMoveUp={canMove(r.key, -1)}
              canMoveDown={canMove(r.key, 1)}
              onMove={(d) => move(r.key, d)}
              onToggleHidden={() => toggleRow(r.key)}
              onHide={() => hideWithUndo(r)}
              onEdit={() => setEditMode(true)}
              onDragEnd={() => {
                if (draftRef.current) commit(draftRef.current);
                updateDraft(null);
              }}
            />
          ))}
        </AnimatePresence>
      </Reorder.Group>
      <AnimatePresence initial={false}>
        {editing && (
          <motion.div key="edit-footer" {...FOLD} transition={transition}>
            <div className="mx-4 mt-1 flex items-center justify-between gap-2 text-[10.5px] text-slate-400">
              <span>Drag to reorder, untick to hide</span>
              <button
                type="button"
                onClick={reset}
                disabled={isDefault}
                className="shrink-0 inline-flex items-center gap-1 h-5 px-1.5 -mr-1.5 rounded text-slate-500 hover:text-slate-800 hover:bg-slate-100 disabled:opacity-40 disabled:pointer-events-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 transition-colors"
              >
                <RotateCcwIcon className="w-3 h-3" />
                Reset
              </button>
            </div>
          </motion.div>
        )}
        {open && !editing && footer ? (
          <motion.div key="footer" className="px-2" {...FOLD} transition={transition}>
            {footer}
          </motion.div>
        ) : null}
      </AnimatePresence>
      <span className="sr-only" aria-live="polite">
        {announcement}
      </span>
    </div>
  );
}

// One Mail or Views row, as it folds, reorders and swaps into its edit form.
function RailRowItem({
  row,
  sectionLabel,
  active,
  editing,
  swapped,
  hidden,
  buttonRef,
  canMoveUp,
  canMoveDown,
  onMove,
  onToggleHidden,
  onHide,
  onEdit,
  onDragEnd,
}: {
  row: RailRow;
  sectionLabel: string;
  active: boolean;
  editing: boolean;
  swapped: boolean;
  hidden: boolean;
  buttonRef?: React.Ref<HTMLButtonElement>;
  canMoveUp: boolean;
  canMoveDown: boolean;
  onMove: (delta: -1 | 1) => void;
  onToggleHidden: () => void;
  onHide: () => void;
  onEdit: () => void;
  onDragEnd: () => void;
}) {
  const transition = useRailTransition();
  const drag = useDragControls();
  return (
    <Reorder.Item
      as="div"
      value={row.key}
      dragListener={false}
      dragControls={drag}
      onDragEnd={onDragEnd}
      layout="position"
      {...FOLD}
      transition={transition}
      whileDrag={{
        scale: 1.02,
        backgroundColor: "rgb(255,255,255)",
        boxShadow: "0 8px 20px -6px rgba(15,23,42,0.18), 0 2px 6px rgba(15,23,42,0.08)",
      }}
      className="relative rounded-md"
    >
      <motion.div
        key={editing ? "edit" : "view"}
        initial={swapped ? { opacity: 0, x: editing ? -6 : 6 } : false}
        animate={{ opacity: 1, x: 0 }}
        transition={transition}
      >
        {editing ? (
          <EditRow
            row={row}
            hidden={hidden}
            drag={drag}
            buttonRef={buttonRef}
            onToggle={onToggleHidden}
            onMove={onMove}
          />
        ) : (
          <MenuRow
            row={row}
            active={active}
            onMove={onMove}
            menu={
              <>
                {row.onMarkAllRead && (
                  <>
                    <PopoverMenuItem icon={<MailOpenIcon className={MENU_ICON} />} onSelect={row.onMarkAllRead}>
                      Mark all as read
                    </PopoverMenuItem>
                    <PopoverMenuSeparator />
                  </>
                )}
                <PopoverMenuItem
                  icon={<ArrowUpIcon className={MENU_ICON} />}
                  disabled={!canMoveUp}
                  onSelect={() => onMove(-1)}
                  trailing={<PopoverMenuKbd>{shortcutLabel("alt+↑")}</PopoverMenuKbd>}
                >
                  Move up
                </PopoverMenuItem>
                <PopoverMenuItem
                  icon={<ArrowDownIcon className={MENU_ICON} />}
                  disabled={!canMoveDown}
                  onSelect={() => onMove(1)}
                  trailing={<PopoverMenuKbd>{shortcutLabel("alt+↓")}</PopoverMenuKbd>}
                >
                  Move down
                </PopoverMenuItem>
                <PopoverMenuItem icon={<EyeOffIcon className={MENU_ICON} />} onSelect={onHide}>
                  Hide from rail
                </PopoverMenuItem>
                <PopoverMenuSeparator />
                <PopoverMenuItem icon={<PencilIcon className={MENU_ICON} />} onSelect={onEdit}>
                  Edit {sectionLabel} rows…
                </PopoverMenuItem>
              </>
            }
          />
        )}
      </motion.div>
    </Reorder.Item>
  );
}

function CollapsibleSection<T extends { id: string }>({
  id,
  label,
  controls,
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
  controls: SectionControls;
  items: T[];
  isActive: (item: T) => boolean;
  hasAccent: (item: T) => boolean;
  emptyText: string;
  searchPlaceholder: string;
  getSearchKey: (item: T) => string;
  renderItem: (item: T) => React.ReactNode;
}) {
  const panelId = React.useId();
  const transition = useRailTransition();
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
  // Folded, only the row you are on stays, even one "Show all" would have tucked away.
  const rows = sectionOpen ? visible : items.filter(isActive);
  const dot = !sectionOpen && items.some((it) => !isActive(it) && hasAccent(it));
  const message = !sectionOpen
    ? null
    : items.length === 0
      ? emptyText
      : filtered.length === 0
        ? "No matches."
        : null;

  return (
    <div className="pb-2">
      <SectionHeader
        label={label}
        panelId={panelId}
        open={sectionOpen}
        onToggle={() => toggleSection(id)}
        dot={dot}
        count={items.length}
        menu={<SectionMenuItems open={sectionOpen} onToggle={() => toggleSection(id)} controls={controls} />}
      />
      <div id={panelId} className="px-2">
        <AnimatePresence initial={false}>
          {sectionOpen && showSearch && (
            <motion.div key="search" {...FOLD} transition={transition}>
              <div className="flex items-center gap-1.5 px-2 h-7 mb-1 rounded-md border border-slate-200 bg-white focus-within:border-sky-400 focus-within:ring-2 focus-within:ring-sky-100 transition-colors">
                <SearchIcon className="w-3 h-3 text-slate-400 shrink-0" />
                <input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key !== "Escape") return;
                    e.stopPropagation();
                    if (search) setSearch("");
                    else e.currentTarget.blur();
                  }}
                  placeholder={searchPlaceholder}
                  aria-label={searchPlaceholder}
                  className="flex-1 min-w-0 h-5 bg-transparent text-[11.5px] text-slate-900 placeholder:text-slate-400 outline-none"
                />
                <AnimatePresence initial={false}>
                  {search && (
                    <motion.button
                      key="clear"
                      type="button"
                      onClick={() => setSearch("")}
                      className="size-4 rounded inline-flex items-center justify-center text-slate-400 hover:text-slate-700 hover:bg-slate-100 shrink-0"
                      aria-label="Clear filter"
                      initial={{ opacity: 0, scale: 0.6 }}
                      animate={{ opacity: 1, scale: 1 }}
                      exit={{ opacity: 0, scale: 0.6 }}
                      transition={transition}
                    >
                      <XIcon className="w-3 h-3" />
                    </motion.button>
                  )}
                </AnimatePresence>
              </div>
            </motion.div>
          )}
          {message && (
            <motion.div key={message} {...FOLD} transition={transition}>
              <div className="px-2 py-1.5 text-[11.5px] text-slate-400">{message}</div>
            </motion.div>
          )}
        </AnimatePresence>

        <div className="space-y-px">
          <AnimatePresence initial={false}>
            {rows.map((it) => (
              <motion.div key={it.id} {...FOLD} transition={transition}>
                {renderItem(it)}
              </motion.div>
            ))}
          </AnimatePresence>
        </div>

        <AnimatePresence initial={false}>
          {sectionOpen && hidden > 0 && (
            <motion.div key="more" {...FOLD} transition={transition}>
              <button
                type="button"
                onClick={() => setExpanded(true)}
                className="w-full h-7 px-2 rounded-md text-[11.5px] text-slate-500 hover:text-slate-900 hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 transition-colors text-left"
              >
                Show all ({hidden} more)
              </button>
            </motion.div>
          )}
          {sectionOpen && expanded && filtered.length > COLLAPSED_VISIBLE && (
            <motion.div key="less" {...FOLD} transition={transition}>
              <button
                type="button"
                onClick={() => setExpanded(false)}
                className="w-full h-7 px-2 rounded-md text-[11.5px] text-slate-400 hover:text-slate-700 hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 transition-colors text-left"
              >
                Show less
              </button>
            </motion.div>
          )}
        </AnimatePresence>
      </div>
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
  "w-full h-7 pl-2 pr-2 rounded-md flex items-center gap-2.5 transition-colors text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200";
// The active row's background is the sliding ActiveMark, not a class.
const ROW_ACTIVE = "text-slate-900";
const ROW_IDLE = "text-slate-600 hover:bg-slate-50 hover:text-slate-900";

// The highlight behind the row you are on; it slides to the next one you pick.
function ActiveMark() {
  const reduce = useReducedMotion();
  return (
    <motion.span
      layoutId="scope-rail-active"
      aria-hidden
      className="absolute inset-0 -z-10 rounded-md bg-slate-100"
      transition={reduce ? { duration: 0 } : { type: "spring", stiffness: 520, damping: 42, mass: 0.8 }}
    />
  );
}

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

// MenuRow: a Mail or Views row with a "…" menu beside it and the same menu on
// right-click. The trigger is the row button's sibling, never its child.
function MenuRow({
  row,
  active,
  onMove,
  menu,
}: {
  row: RailRow;
  active: boolean;
  onMove: (delta: -1 | 1) => void;
  menu: React.ReactNode;
}) {
  const rowMenu = useAnchoredMenu();
  const body = (
    <div className="group/folder relative" onContextMenu={rowMenu.onContextMenu}>
      <button
        type="button"
        data-rail-row={row.key}
        onClick={row.onOpen}
        onKeyDown={(e) => {
          if (!e.altKey || (e.key !== "ArrowUp" && e.key !== "ArrowDown")) return;
          e.preventDefault();
          e.stopPropagation();
          onMove(e.key === "ArrowUp" ? -1 : 1);
        }}
        aria-current={active ? "true" : undefined}
        className={cn(
          "relative isolate pr-7 md:pr-2",
          ROW,
          active ? ROW_ACTIVE : ROW_IDLE,
          rowMenu.open && !active && "bg-slate-50",
        )}
        title={row.tooltip ? undefined : row.label}
      >
        {active && <ActiveMark />}
        <span className={cn("shrink-0 transition-colors", active ? "text-slate-800" : "text-slate-400 group-hover/folder:text-slate-600")}>
          {row.icon}
        </span>
        <span className={cn("truncate min-w-0 flex-1 text-[12.5px]", active && "font-medium")}>{row.label}</span>
        {row.count !== undefined && row.count !== null && (
          <span
            className={cn(
              "md:group-hover/folder:invisible md:group-focus-within/folder:invisible",
              rowMenu.open && "md:invisible",
            )}
          >
            <Count value={row.count} accent={row.accent} active={active} />
          </span>
        )}
      </button>
      <PopoverMenu align="end" {...rowMenu.menuProps}>
        <PopoverMenuTrigger asChild>
          <button
            type="button"
            aria-label={`${row.label} ${row.noun ? `${row.noun} ` : ""}actions`}
            className="absolute right-1 top-1 size-5 rounded inline-flex items-center justify-center text-slate-500 hover:text-slate-900 hover:bg-slate-200/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 transition-[opacity,color,background-color] opacity-100 md:opacity-0 md:group-hover/folder:opacity-100 md:group-focus-within/folder:opacity-100 data-[state=open]:opacity-100 data-[state=open]:bg-slate-200/70"
          >
            <MoreHorizontalIcon className="w-3.5 h-3.5" />
          </button>
        </PopoverMenuTrigger>
        <PopoverMenuContent minWidth={200}>{menu}</PopoverMenuContent>
      </PopoverMenu>
    </div>
  );
  if (!row.tooltip) return body;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div>{body}</div>
      </TooltipTrigger>
      <TooltipContent sideOffset={6} className="max-w-72">
        {row.tooltip}
      </TooltipContent>
    </Tooltip>
  );
}

// EditRow: a row while its section is edited. The grip drags it (or moves it
// with the arrow keys); the rest is a checkbox, checked meaning shown.
function EditRow({
  row,
  hidden,
  drag,
  buttonRef,
  onToggle,
  onMove,
}: {
  row: RailRow;
  hidden: boolean;
  drag: DragControls;
  buttonRef?: React.Ref<HTMLButtonElement>;
  onToggle: () => void;
  onMove: (delta: -1 | 1) => void;
}) {
  const moveKey = (e: React.KeyboardEvent, from: "row" | "grip") => {
    if (from === "row" && !e.altKey) return;
    if (e.key !== "ArrowUp" && e.key !== "ArrowDown") return;
    e.preventDefault();
    e.stopPropagation();
    onMove(e.key === "ArrowUp" ? -1 : 1);
  };
  return (
    <div className="h-7 flex items-center rounded-md hover:bg-slate-50 transition-colors">
      <button
        type="button"
        onPointerDown={(e) => drag.start(e)}
        onKeyDown={(e) => moveKey(e, "grip")}
        aria-label={`Move ${row.label}: drag, or press the up and down arrow keys`}
        className="size-5 ml-0.5 shrink-0 flex items-center justify-center rounded cursor-grab active:cursor-grabbing text-slate-300 hover:text-slate-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200 focus-visible:text-slate-600 touch-none"
      >
        <GripVerticalIcon className="w-3 h-3" />
      </button>
      <button
        ref={buttonRef}
        type="button"
        role="checkbox"
        aria-checked={!hidden}
        data-rail-row={row.key}
        onClick={onToggle}
        onKeyDown={(e) => moveKey(e, "row")}
        className="flex-1 min-w-0 h-7 pl-1 pr-2 flex items-center gap-2.5 rounded-md text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-200"
      >
        <motion.span
          key={hidden ? "off" : "on"}
          className="flex"
          initial={{ scale: 0.7 }}
          animate={{ scale: 1 }}
          transition={{ type: "spring", stiffness: 600, damping: 26 }}
        >
          <CheckSquare checked={!hidden} tone="sky" />
        </motion.span>
        <span className={cn("shrink-0 transition-colors", hidden ? "text-slate-300" : "text-slate-400")}>
          {row.icon}
        </span>
        <span
          className={cn(
            "truncate min-w-0 flex-1 text-[12.5px] transition-colors",
            hidden ? "text-slate-400" : "text-slate-700",
          )}
        >
          {row.label}
        </span>
      </button>
    </div>
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
      data-rail-row
      onClick={onClick}
      // A scope, not a page, so "true" rather than the nav's "page".
      aria-current={active ? "true" : undefined}
      className={cn("group/item relative isolate", ROW, active ? ROW_ACTIVE : ROW_IDLE)}
      title={hideNativeTitle ? undefined : label}
    >
      {active && <ActiveMark />}
      <span className={cn("shrink-0 transition-colors", active ? "text-slate-800" : "text-slate-400 group-hover/item:text-slate-600")}>
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
