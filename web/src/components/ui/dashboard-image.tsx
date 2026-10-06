import React from "react";
import { useDashboardImage } from "@/hooks/useDashboardImage";

export function DashboardImage({ src, ref, ...props }: Omit<React.ComponentPropsWithRef<"img">, "srcSet">) {
    const image = React.useRef<HTMLImageElement | null>(null);
    const { url, error } = useDashboardImage(src);
    React.useEffect(() => {
        if (error) image.current?.dispatchEvent(new Event("error"));
    }, [error]);
    return <img {...props} src={url} ref={(element) => {
        image.current = element;
        if (typeof ref === "function") ref(element);
        else if (ref) ref.current = element;
    }} />;
}
