import * as React from "react"

import { cn } from "@/lib/utils"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "file:text-foreground placeholder:text-subtle-foreground border-input h-8 w-full min-w-0 rounded-md border bg-card px-2.5 py-1 text-[13px] transition-[border-color,box-shadow] outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:text-[13px] file:font-medium hover:border-border-strong disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white/[0.02]",
        "focus-visible:border-[color-mix(in_oklab,var(--foreground)_40%,transparent)] focus-visible:ring-[3px] focus-visible:ring-[color-mix(in_oklab,var(--foreground)_8%,transparent)]",
        "aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive",
        className
      )}
      {...props}
    />
  )
}

export { Input }
