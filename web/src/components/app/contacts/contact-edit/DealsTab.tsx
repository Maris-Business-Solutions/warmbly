// Deals tab: the contact's deals, with stage moves and a quick add. In
// provider mode these are the CRM's deals and every change is written there.

import React from "react";
import { Link } from "react-router-dom";
import toast from "react-hot-toast";
import { CheckIcon, ChevronDownIcon, CircleDollarSignIcon, Loader2Icon, MegaphoneIcon, PlusIcon, XIcon } from "lucide-react";
import { AnimatePresence, motion } from "framer-motion";
import { NumberInput, TextInput } from "@/components/ui/field";
import { PopoverMenu, PopoverMenuContent, PopoverMenuItem, PopoverMenuTrigger } from "@/components/ui/popover-menu";
import DealStagePicker from "@/components/app/crm/DealStagePicker";
import { CrmBadge, CrmSyncedAt, OpenInCrm } from "@/components/app/crm/crmProviders";
import { externalOwnerName } from "@/components/app/crm/crmModeUtils";
import { crmErrorMessage, latestSyncedAt } from "@/components/app/crm/crmModeUtils";
import useCrmProvider from "@/hooks/useCrmProvider";
import useContactDeals from "@/lib/api/hooks/app/contacts/useContactDeals";
import useCreateDeal from "@/lib/api/hooks/app/crm/deals/useCreateDeal";
import useUpdateDeal from "@/lib/api/hooks/app/crm/deals/useUpdateDeal";
import usePipelines from "@/lib/api/hooks/app/crm/pipelines/usePipelines";
import type Deal from "@/lib/api/models/app/crm/Deal";
import type { DealWrite } from "@/lib/api/models/app/crm/Deal";
import type { Stage } from "@/lib/api/models/app/crm/Pipeline";
import { fmtAbsolute } from "./format";

const DEAL_STATUS: Record<Deal["status"], { label: string; cls: string; dot: string }> = {
    open: { label: "Open", cls: "text-slate-600", dot: "bg-slate-400" },
    won: { label: "Won", cls: "text-emerald-700", dot: "bg-emerald-500" },
    lost: { label: "Lost", cls: "text-red-700", dot: "bg-red-500" },
};

export default function DealsTab({ contactId, defaultName }: { contactId: string; defaultName: string }) {
    const { isExternal, crm } = useCrmProvider();
    const deals = useContactDeals(contactId);
    const pipelines = usePipelines();
    const update = useUpdateDeal();
    const [adding, setAdding] = React.useState(false);

    const list = React.useMemo(() => (Array.isArray(deals.data) ? deals.data : []), [deals.data]);
    const stagesOf = React.useCallback(
        (pipelineId: string): Stage[] => {
            const p = pipelines.data?.find((x) => x.id === pipelineId);
            return p ? [...(p.stages ?? [])].sort((a, b) => a.position - b.position) : [];
        },
        [pipelines.data],
    );
    const syncedAt = isExternal ? latestSyncedAt(list) : undefined;
    const openValue = list.filter((d) => d.status === "open").reduce((n, d) => n + (d.value ?? 0), 0);

    async function moveDeal(deal: Deal, stageId: string) {
        try {
            await toast.promise(update.mutateAsync({ id: deal.id, data: { stage_id: stageId } as DealWrite }), {
                loading: isExternal ? `Moving in ${crm.name}…` : "Moving…",
                success: "Moved",
                error: (e: unknown) => crmErrorMessage(e),
            });
        } catch {
            /* surfaced */
        }
    }

    return (
        <div className="space-y-3">
            <div className="flex items-center gap-2">
                <h2 className="text-[10px] uppercase tracking-[0.14em] font-semibold text-slate-500">Deals</h2>
                {isExternal && <CrmBadge provider={crm.id} />}
                {list.length > 0 && (
                    <span className="text-[10.5px] text-slate-400 tabular-nums">
                        {list.length}
                        {openValue > 0 ? ` · ${money(openValue, list[0]?.currency)} open` : ""}
                    </span>
                )}
                <div className="ml-auto flex items-center gap-1">
                    <Link
                        to="/app/crm/deals"
                        className="h-7 px-2 rounded-md text-[12px] text-slate-500 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center transition-colors"
                    >
                        All deals
                    </Link>
                    <button
                        type="button"
                        onClick={() => setAdding((a) => !a)}
                        className="h-7 px-2 rounded-md border border-slate-200 hover:border-slate-300 bg-white text-[12px] text-slate-700 hover:text-slate-900 inline-flex items-center gap-1 transition-colors"
                    >
                        {adding ? <XIcon className="w-3 h-3" /> : <PlusIcon className="w-3 h-3" />}
                        {adding ? "Cancel" : "New deal"}
                    </button>
                </div>
            </div>

            <AnimatePresence initial={false}>
                {adding && (
                    <motion.div
                        key="new-deal"
                        initial={{ height: 0, opacity: 0 }}
                        animate={{ height: "auto", opacity: 1 }}
                        exit={{ height: 0, opacity: 0 }}
                        transition={{ duration: 0.18, ease: [0.32, 0.72, 0, 1] }}
                        className="overflow-hidden"
                    >
                        <NewDealForm
                            contactId={contactId}
                            defaultName={defaultName}
                            external={isExternal}
                            onDone={() => setAdding(false)}
                        />
                    </motion.div>
                )}
            </AnimatePresence>

            {deals.isPending ? (
                <div className="space-y-1.5">
                    {[0, 1].map((i) => (
                        <div key={i} className="h-14 rounded-md bg-slate-100 animate-pulse" />
                    ))}
                </div>
            ) : deals.isError ? (
                <div className="rounded-md border border-red-200 bg-red-50/50 px-3 py-2.5 text-[11.5px] text-red-700">
                    Failed to load deals.
                </div>
            ) : list.length === 0 ? (
                <div className="rounded-md border border-dashed border-slate-200 px-3 py-8 text-center">
                    <CircleDollarSignIcon className="w-4 h-4 text-slate-300 mx-auto mb-1.5" />
                    <p className="text-[11.5px] text-slate-500">
                        {isExternal ? `No deals for this contact in ${crm.name}.` : "No deals for this contact yet."}
                    </p>
                </div>
            ) : (
                <div className="space-y-1.5">
                    {list.map((d) => (
                        <DealRow key={d.id} deal={d} stages={stagesOf(d.pipeline_id)} external={isExternal} onMove={moveDeal} />
                    ))}
                </div>
            )}

            {syncedAt && <CrmSyncedAt provider={crm.id} at={syncedAt} />}
        </div>
    );
}

function DealRow({
    deal,
    stages,
    external,
    onMove,
}: {
    deal: Deal;
    stages: Stage[];
    external: boolean;
    onMove: (deal: Deal, stageId: string) => void;
}) {
    const { crm } = useCrmProvider();
    const st = DEAL_STATUS[deal.status] ?? DEAL_STATUS.open;
    const canMove = deal.status === "open" && stages.length > 0;
    const owner = externalOwnerName(deal.external);
    return (
        <div className="rounded-md border border-slate-200 bg-white px-3 py-2">
            <div className="flex items-center gap-2 min-w-0">
                <span className="text-[12.5px] font-medium text-slate-900 truncate flex-1 min-w-0" title={deal.name}>
                    {deal.name}
                </span>
                {deal.value != null && (
                    <span className="font-mono text-[11.5px] text-emerald-700 tabular-nums shrink-0">
                        {money(deal.value, deal.currency)}
                    </span>
                )}
                {external && <OpenInCrm external={deal.external} compact label={`Open deal in ${crm.name}`} />}
            </div>
            <div className="mt-1 flex items-center gap-2 min-w-0 flex-wrap">
                <span className={`inline-flex items-center gap-1 text-[11px] ${st.cls}`}>
                    <span className={`size-1.5 rounded-full ${st.dot}`} />
                    {st.label}
                </span>
                {canMove ? (
                    <StageMenu stages={stages} value={deal.stage_id} onChange={(id) => onMove(deal, id)} />
                ) : deal.stage?.name ? (
                    <span className="text-[11px] text-slate-500 truncate">{deal.stage.name}</span>
                ) : null}
                {deal.expected_close_date && (
                    <span className="text-[11px] text-slate-400">Closes {fmtAbsolute(deal.expected_close_date)}</span>
                )}
                {owner && <span className="text-[11px] text-slate-400 truncate">Owner {owner}</span>}
                {deal.campaign_name && (
                    <Link
                        to={`/app/campaigns/${deal.campaign_id}`}
                        className="ml-auto inline-flex items-center gap-1 text-[11px] text-slate-400 hover:text-sky-700 min-w-0 transition-colors"
                    >
                        <MegaphoneIcon className="w-3 h-3 shrink-0" />
                        <span className="truncate max-w-[160px]">{deal.campaign_name}</span>
                    </Link>
                )}
            </div>
        </div>
    );
}

function StageMenu({ stages, value, onChange }: { stages: Stage[]; value?: string; onChange: (id: string) => void }) {
    const [open, setOpen] = React.useState(false);
    const cur = stages.find((s) => s.id === value);
    return (
        <PopoverMenu open={open} onOpenChange={setOpen} align="start">
            <PopoverMenuTrigger asChild>
                <button
                    type="button"
                    className="h-6 px-1.5 -ml-0.5 rounded-md inline-flex items-center gap-1 min-w-0 max-w-[180px] text-[11px] text-slate-600 hover:text-slate-900 hover:bg-slate-100 transition-colors"
                >
                    <span className="size-1.5 rounded-full shrink-0" style={{ backgroundColor: cur?.color || "#94a3b8" }} />
                    <span className="truncate">{cur?.name ?? "Stage"}</span>
                    <ChevronDownIcon className="w-3 h-3 text-slate-400 shrink-0" />
                </button>
            </PopoverMenuTrigger>
            <PopoverMenuContent minWidth={180} className="max-h-56 overflow-y-auto">
                {stages.map((s) => (
                    <PopoverMenuItem
                        key={s.id}
                        selected={s.id === value}
                        onSelect={() => {
                            if (s.id !== value) onChange(s.id);
                        }}
                        icon={<span className="size-2 rounded-full block" style={{ backgroundColor: s.color || "#94a3b8" }} />}
                    >
                        {s.name}
                    </PopoverMenuItem>
                ))}
            </PopoverMenuContent>
        </PopoverMenu>
    );
}

function NewDealForm({
    contactId,
    defaultName,
    external,
    onDone,
}: {
    contactId: string;
    defaultName: string;
    external: boolean;
    onDone: () => void;
}) {
    const { crm } = useCrmProvider();
    const pipelines = usePipelines();
    const create = useCreateDeal();
    const [name, setName] = React.useState(defaultName);
    const [value, setValue] = React.useState(0);
    const [target, setTarget] = React.useState<{ pipelineId?: string; stageId?: string }>({});

    // Default to the first pipeline's first stage once pipelines load.
    React.useEffect(() => {
        if (target.pipelineId) return;
        const first = pipelines.data?.[0];
        if (!first) return;
        const stage = [...(first.stages ?? [])].sort((a, b) => a.position - b.position)[0];
        setTarget({ pipelineId: first.id, stageId: stage?.id });
    }, [pipelines.data, target.pipelineId]);

    const ready = !!name.trim() && !!target.pipelineId && !!target.stageId;

    async function submit() {
        if (!ready || create.isPending) return;
        const data: DealWrite = {
            name: name.trim(),
            pipeline_id: target.pipelineId,
            stage_id: target.stageId,
            contact_id: contactId,
            currency: "USD",
        };
        if (value > 0) data.value = value;
        try {
            await toast.promise(create.mutateAsync(data), {
                loading: external ? `Creating in ${crm.name}…` : "Creating deal…",
                success: "Deal created",
                error: (e: unknown) => crmErrorMessage(e),
            });
            onDone();
        } catch {
            /* surfaced */
        }
    }

    return (
        <div className="rounded-md border border-slate-200 bg-slate-50/40 p-3 space-y-2">
            <div className="grid grid-cols-[1fr_120px] gap-2">
                <div>
                    <p className="mb-1 text-[10px] font-medium uppercase tracking-[0.14em] text-slate-400">Name</p>
                    <TextInput value={name} onChange={setName} placeholder="Deal name" className="w-full" autoFocus />
                </div>
                <div>
                    <p className="mb-1 text-[10px] font-medium uppercase tracking-[0.14em] text-slate-400">Value</p>
                    <NumberInput value={value} onChange={setValue} min={0} step={100} className="w-full" />
                </div>
            </div>
            <DealStagePicker
                pipelineId={target.pipelineId}
                stageId={target.stageId}
                onChange={(next) => setTarget(next)}
            />
            <div className="flex items-center gap-2">
                <span className="text-[11px] text-slate-400">
                    {external ? `Created in ${crm.name} and linked to this contact.` : "Linked to this contact."}
                </span>
                <button
                    type="button"
                    onClick={submit}
                    disabled={!ready || create.isPending}
                    title={!ready ? "Name the deal and pick a stage" : undefined}
                    className="ml-auto h-7 px-3 rounded-md bg-sky-600 hover:bg-sky-700 text-white text-[12px] font-medium inline-flex items-center gap-1.5 transition-colors disabled:bg-slate-200 disabled:text-slate-500"
                >
                    {create.isPending ? <Loader2Icon className="w-3 h-3 animate-spin" /> : <CheckIcon className="w-3 h-3" />}
                    Create deal
                </button>
            </div>
        </div>
    );
}

function money(n: number, currency = "USD") {
    try {
        return new Intl.NumberFormat("en-US", { style: "currency", currency: currency || "USD", maximumFractionDigits: 0 }).format(n);
    } catch {
        return `$${Math.round(n).toLocaleString()}`;
    }
}
