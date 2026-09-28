import React from "react";
import {
    CopyCheckIcon,
    DownloadIcon,
    FileSpreadsheetIcon,
    Loader2Icon,
    MailIcon,
    TableIcon,
    UploadCloudIcon,
    WandSparklesIcon,
} from "lucide-react";

import { downloadBlob } from "@/lib/api/client/app/contacts/exportContacts";
import type { ContactImport } from "@/lib/api/models/app/contacts/ContactImport";
import { sampleCSV } from "../importShared";
import RecentImports from "./RecentImports";

const ACCEPT = ".csv,.tsv,.txt,.xlsx,.xlsm";

export default function UploadStep({
    onFile,
    busy,
    uploadFraction,
    fileName,
    onOpenImport,
}: {
    onFile: (f: File) => void;
    busy: boolean;
    /** 0..1 while the file uploads, then null while the server reads it. */
    uploadFraction: number | null;
    fileName?: string;
    onOpenImport: (imp: ContactImport) => void;
}) {
    const inputRef = React.useRef<HTMLInputElement>(null);
    const [dragging, setDragging] = React.useState(false);

    function onDrop(e: React.DragEvent<HTMLDivElement>) {
        e.preventDefault();
        setDragging(false);
        if (busy) return;
        const f = e.dataTransfer.files?.[0];
        if (f) onFile(f);
    }

    return (
        <div className="space-y-4">
            <input
                ref={inputRef}
                type="file"
                accept={ACCEPT}
                className="hidden"
                onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) onFile(f);
                    e.target.value = "";
                }}
            />
            <div
                role="button"
                tabIndex={0}
                aria-label="Choose a file to import"
                onClick={() => !busy && inputRef.current?.click()}
                onKeyDown={(e) => {
                    if (!busy && (e.key === "Enter" || e.key === " ")) {
                        e.preventDefault();
                        inputRef.current?.click();
                    }
                }}
                onDragOver={(e) => {
                    e.preventDefault();
                    setDragging(true);
                }}
                onDragLeave={() => setDragging(false)}
                onDrop={onDrop}
                className={`relative rounded-lg border-2 border-dashed px-6 py-9 text-center transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sky-100 ${
                    busy ? "cursor-default border-slate-200 bg-slate-50/50" : "cursor-pointer"
                } ${
                    dragging
                        ? "border-sky-400 bg-sky-50/60"
                        : busy
                          ? ""
                          : "border-slate-200 hover:border-slate-300 hover:bg-slate-50/50"
                }`}
            >
                {busy ? (
                    <div className="max-w-[320px] mx-auto">
                        <Loader2Icon className="w-7 h-7 mx-auto text-sky-600 animate-spin" />
                        <p className="text-[12.5px] text-slate-900 font-medium mt-3 truncate">{fileName}</p>
                        <p className="text-[11.5px] text-slate-500 mt-0.5">
                            {uploadFraction !== null && uploadFraction < 1
                                ? `Uploading… ${Math.round(uploadFraction * 100)}%`
                                : "Reading every row…"}
                        </p>
                        <div className="mt-3 h-1.5 rounded-full bg-slate-100 overflow-hidden">
                            {uploadFraction !== null && uploadFraction < 1 ? (
                                <div
                                    className="h-full bg-sky-500 rounded-full transition-[width] duration-300"
                                    style={{ width: `${Math.max(3, uploadFraction * 100)}%` }}
                                />
                            ) : (
                                <div className="h-full w-1/3 bg-sky-500 rounded-full progress-sweep" />
                            )}
                        </div>
                    </div>
                ) : (
                    <>
                        <div
                            className={`size-11 mx-auto rounded-full flex items-center justify-center transition-colors ${
                                dragging ? "bg-sky-100 text-sky-700" : "bg-slate-100 text-slate-500"
                            }`}
                        >
                            <UploadCloudIcon className="w-5 h-5" />
                        </div>
                        <p className="text-[13px] text-slate-900 font-medium mt-3">
                            {dragging ? "Drop it to start" : "Drag a file here, or click to browse"}
                        </p>
                        <div className="mt-2 flex items-center justify-center gap-1.5">
                            {["CSV", "TSV", "XLSX"].map((f) => (
                                <span
                                    key={f}
                                    className="inline-flex items-center gap-1 h-5 px-1.5 rounded bg-white border border-slate-200 text-[10.5px] font-medium text-slate-600"
                                >
                                    <FileSpreadsheetIcon className="w-3 h-3 text-emerald-600" />
                                    {f}
                                </span>
                            ))}
                            <span className="text-[11px] text-slate-400">up to 50 MB · 50,000 rows</span>
                        </div>
                    </>
                )}
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                {[
                    { icon: MailIcon, title: "One column of emails", text: "The only thing a file must have. Everything else is optional." },
                    { icon: WandSparklesIcon, title: "Columns map themselves", text: "Headers are matched to fields; you can change any of them." },
                    { icon: CopyCheckIcon, title: "Checked before it runs", text: "See new, existing, duplicate and invalid rows before anything is written." },
                ].map((tip) => (
                    <div key={tip.title} className="rounded-md border border-slate-200 bg-slate-50/40 p-2.5">
                        <div className="flex items-center gap-1.5">
                            <tip.icon className="w-3.5 h-3.5 text-sky-600 shrink-0" />
                            <p className="text-[11.5px] text-slate-900 font-medium">{tip.title}</p>
                        </div>
                        <p className="text-[11px] text-slate-500 leading-snug mt-1">{tip.text}</p>
                    </div>
                ))}
            </div>

            <button
                type="button"
                onClick={() => downloadBlob(new Blob([sampleCSV()], { type: "text/csv;charset=utf-8" }), "warmbly-contacts-sample.csv")}
                className="inline-flex items-center gap-1.5 text-[11.5px] text-slate-600 hover:text-slate-900 transition-colors"
            >
                <TableIcon className="w-3 h-3" />
                Starting from scratch? Download a sample file
                <DownloadIcon className="w-3 h-3" />
            </button>

            <RecentImports onOpen={onOpenImport} />
        </div>
    );
}
