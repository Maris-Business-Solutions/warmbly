import * as React from "react"

import { cn } from "@/lib/utils"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "border-input placeholder:text-subtle-foreground hover:border-border-strong focus-visible:border-[color-mix(in_oklab,var(--foreground)_40%,transparent)] focus-visible:ring-[3px] focus-visible:ring-[color-mix(in_oklab,var(--foreground)_8%,transparent)] aria-invalid:border-destructive flex field-sizing-content min-h-16 w-full rounded-md border bg-card px-2.5 py-2 text-[13px] leading-relaxed transition-[border-color,box-shadow] outline-none disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white/[0.02]",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
