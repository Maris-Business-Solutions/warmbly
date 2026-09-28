import React from "react";
import { companyLogosEnabled, companyLogoUrl, markLogoFailed } from "@/lib/companyLogo";
import useAuthConfig from "@/lib/api/hooks/auth/useAuthConfig";

// CompanyLogo shows a company domain's logo, and whatever `fallback` is when
// the domain has none (or logos are off). It never shows a broken image.
export default function CompanyLogo({
    domain,
    className,
    fallback = null,
}: {
    domain: string;
    className?: string;
    fallback?: React.ReactNode;
}) {
    const [broken, setBroken] = React.useState(false);
    const [loaded, setLoaded] = React.useState(false);
    const enabled = companyLogosEnabled(useAuthConfig().data?.self_hosted);
    const src = broken ? "" : companyLogoUrl(domain, enabled);
    React.useEffect(() => {
        setBroken(false);
        setLoaded(false);
    }, [domain]);
    if (!src) return <>{fallback}</>;
    // Invisible rather than display:none until it arrives: a lazy image that is
    // not laid out is never fetched, and would leave the fallback up for good.
    return (
        <span className="relative inline-flex items-center justify-center shrink-0">
            {!loaded && fallback}
            <img
                src={src}
                alt=""
                loading="lazy"
                decoding="async"
                referrerPolicy="no-referrer"
                draggable={false}
                onLoad={() => setLoaded(true)}
                onError={() => {
                    markLogoFailed(domain);
                    setBroken(true);
                }}
                className={`${className ?? ""} ${loaded ? "" : "absolute inset-0 m-auto opacity-0 pointer-events-none"}`}
            />
        </span>
    );
}
