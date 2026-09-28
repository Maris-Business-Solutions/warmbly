// The result of a synchronous import (the Google Sheets "sync now"). The file
// import runs in the background and reports through RunStep instead.

import {
    AlertTriangleIcon,
    CheckCircle2Icon,
    CircleSlashIcon,
    DownloadIcon,
    RefreshCwIcon,
    UserPlusIcon,
    XCircleIcon,
} from "lucide-react";
import type { ImportResult } from "@/lib/api/client/app/contacts/importContacts";
import { downloadBlob } from "@/lib/api/client/app/contacts/exportContacts";
import StatCard from "./StatCard";

export default function ResultStep({
    result,
    filename,
    pinnedSegments,
}: {
    result: ImportResult;
    filename: string;
    // Names of the segments every imported, updated and skipped row was
    // pinned into, so the wizard confirms the membership it just wrote.
    pinnedSegments?: string[];
}) {
    function downloadErrors() {
        if (!result.errors || result.errors.length === 0) return;
        const rows = [["line", "email", "reason"]];
        for (const e of result.errors) {
            rows.push([String(e.line), e.email ?? "", e.reason.replace(/\r?\n/g, " ")]);
        }
        const csv = rows
            .map((r) =>
                r
                    .map((v) => (/[,"\n]/.test(v) ? `"${v.replace(/"/g, '""')}"` : v))
                    .join(","),
            )
            .join("\n");
        const blob = new Blob(["﻿" + csv], { type: "text/csv;charset=utf-8" });
        downloadBlob(blob, filename.replace(/\.[^.]+$/, "") + "-errors.csv");
    }

    return (
        <div className="space-y-4">
            <div className="flex items-center gap-3">
                {result.failed === 0 ? (
                    <CheckCircle2Icon className="w-8 h-8 text-emerald-600 shrink-0" />
                ) : (
                    <AlertTriangleIcon className="w-8 h-8 text-amber-600 shrink-0" />
                )}
                <div className="flex-1">
                    <p className="text-[13.5px] text-slate-900 font-semibold">
                        {result.failed === 0 ? "Import complete" : "Import finished with errors"}
                    </p>
                    <p className="text-[11.5px] text-slate-500 leading-snug mt-0.5">
                        Processed {result.total.toLocaleString()} rows in{" "}
                        {durationText(result.started_at, result.ended_at)}.
                        {result.segments_pinned && pinnedSegments && pinnedSegments.length > 0 && (
                            <> Pinned into {pinnedSegments.join(", ")}.</>
                        )}
                    </p>
                </div>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                <StatCard label="Imported" value={result.imported} accent="emerald" icon={UserPlusIcon} />
                <StatCard label="Updated" value={result.updated} accent="sky" icon={RefreshCwIcon} />
                <StatCard label="Skipped" value={result.skipped} accent="slate" icon={CircleSlashIcon} />
                <StatCard label="Failed" value={result.failed} accent={result.failed > 0 ? "red" : "slate"} icon={XCircleIcon} />
            </div>

            {result.segments_pinned === false && (
                <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 flex items-start gap-2">
                    <AlertTriangleIcon className="w-3.5 h-3.5 mt-px shrink-0 text-amber-600" />
                    <div className="min-w-0">
                        <p className="text-[12.5px] font-medium text-amber-900">
                            The contacts are in, but not in the segment
                        </p>
                        <p className="text-[11.5px] text-amber-800/90 leading-relaxed mt-0.5">
                            The rows imported; the membership write did not. The reason is in the notes below. Select
                            them in your contact list and use <span className="font-medium">Segment</span> to add them,
                            or run the import again.
                        </p>
                    </div>
                </div>
            )}

            {result.quality?.flagged && (
                <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2.5 flex items-start gap-2">
                    <AlertTriangleIcon className="w-3.5 h-3.5 mt-px shrink-0 text-amber-600" />
                    <div className="min-w-0">
                        <p className="text-[12.5px] font-medium text-amber-900">This list looks low quality</p>
                        <p className="text-[11.5px] text-amber-800/90 leading-relaxed mt-0.5">
                            {result.quality.summary} They are imported, but sending to them risks the reputation of
                            every mailbox in this workspace. Clean the list before launching a campaign with it.
                        </p>
                    </div>
                </div>
            )}

            {result.errors && result.errors.length > 0 && (
                <div className="rounded-md border border-slate-200 overflow-hidden">
                    <div className="px-3 h-9 border-b border-slate-200 bg-slate-50/60 flex items-center gap-2">
                        <span className="text-[11px] uppercase tracking-[0.14em] text-slate-500 font-medium">
                            {result.failed === 0 ? "Notes" : "Errors"}
                        </span>
                        <span className="text-[11px] text-slate-500">
                            {result.errors_truncated
                                ? `${result.errors.length.toLocaleString()} of ${result.failed.toLocaleString()}`
                                : result.errors.length.toLocaleString()}
                        </span>
                        <button
                            type="button"
                            onClick={downloadErrors}
                            className="ml-auto h-6 px-2 rounded text-[11px] text-slate-700 hover:text-slate-900 hover:bg-slate-100 inline-flex items-center gap-1 transition-colors"
                        >
                            <DownloadIcon className="w-3 h-3" />
                            Download errors
                        </button>
                    </div>
                    <div className="max-h-56 overflow-y-auto">
                        <table className="w-full text-left">
                            <thead className="bg-white sticky top-0">
                                <tr className="border-b border-slate-100">
                                    <th className="px-3 py-1.5 text-[10px] font-medium text-slate-400 uppercase tracking-[0.14em] w-12">Line</th>
                                    <th className="hidden md:table-cell px-3 py-1.5 text-[10px] font-medium text-slate-400 uppercase tracking-[0.14em]">Email</th>
                                    <th className="px-3 py-1.5 text-[10px] font-medium text-slate-400 uppercase tracking-[0.14em]">Reason</th>
                                </tr>
                            </thead>
                            <tbody>
                                {result.errors.slice(0, 200).map((e, i) => (
                                    <tr key={i} className="border-b border-slate-100 last:border-b-0">
                                        <td className="px-3 py-1.5 text-[11px] text-slate-500 font-mono">
                                            {e.line > 0 ? e.line : <span className="text-slate-300">—</span>}
                                        </td>
                                        <td className="hidden md:table-cell px-3 py-1.5 text-[11.5px] text-slate-700 truncate max-w-[180px]">
                                            {e.email || <span className="text-slate-300">—</span>}
                                        </td>
                                        <td className="px-3 py-1.5 text-[11.5px] text-slate-700 leading-snug">{e.reason}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}
        </div>
    );
}

function durationText(start: string | Date, end: string | Date): string {
    const s = new Date(start).getTime();
    const e = new Date(end).getTime();
    if (Number.isNaN(s) || Number.isNaN(e)) return "—";
    const ms = e - s;
    if (ms < 1000) return `${ms} ms`;
    const sec = ms / 1000;
    if (sec < 60) return `${sec.toFixed(1)} s`;
    return `${(sec / 60).toFixed(1)} min`;
}
