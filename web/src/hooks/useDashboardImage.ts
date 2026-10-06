import { useEffect, useState } from "react";
import { acquireDashboardImage, directDashboardImage } from "@/lib/dashboardImage";

export function useDashboardImage(src?: string | null): { url?: string; error?: boolean } {
    const [result, setResult] = useState<{ source: string; url?: string; error?: boolean }>();
    const direct = !!src && directDashboardImage(src);
    useEffect(() => {
        if (!src || direct) return;
        const lease = acquireDashboardImage(src);
        let active = true;
        lease.promise.then(
            (url) => { if (active) setResult({ source: src, url }); },
            () => { if (active) setResult({ source: src, error: true }); },
        );
        return () => { active = false; lease.release(); };
    }, [src, direct]);
    return direct ? { url: src ?? undefined } : result && result.source === src ? result : {};
}
