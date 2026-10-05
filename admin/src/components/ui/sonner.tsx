// Toaster wrapper. The admin app only ships sonner (not react-hot-toast)
// to keep the bundle tighter; if you need toast() somewhere, import it
// from "sonner" directly.

import {
    CircleCheckIcon,
    InfoIcon,
    Loader2Icon,
    OctagonXIcon,
    TriangleAlertIcon,
} from "lucide-react";
import { Toaster as Sonner, type ToasterProps } from "sonner";
import { useTheme } from "@/lib/theme";

export function Toaster(props: ToasterProps) {
    const { resolved } = useTheme();
    return (
        <Sonner
            theme={resolved}
            className="toaster group"
            icons={{
                success: <CircleCheckIcon className="size-4" />,
                info: <InfoIcon className="size-4" />,
                warning: <TriangleAlertIcon className="size-4" />,
                error: <OctagonXIcon className="size-4" />,
                loading: <Loader2Icon className="size-4 animate-spin" />,
            }}
            style={
                {
                    "--normal-bg": "var(--popover)",
                    "--normal-text": "var(--popover-foreground)",
                    "--normal-border": "var(--border)",
                    "--border-radius": "var(--radius)",
                } as React.CSSProperties
            }
            {...props}
        />
    );
}
