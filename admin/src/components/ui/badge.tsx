import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center justify-center rounded-full border px-2 py-px text-[11.5px] leading-[18px] font-medium w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none focus-visible:ring-2 focus-visible:ring-ring/40 transition-colors overflow-hidden",
  {
    variants: {
      variant: {
        default:
          "border-border-strong bg-[var(--admin-accent-weak)] text-foreground",
        secondary:
          "border-transparent bg-muted text-muted-foreground [a&]:hover:text-foreground",
        destructive:
          "border-red-500/25 bg-red-500/10 text-red-700 dark:text-red-400",
        outline:
          "border-border-strong text-foreground [a&]:hover:bg-accent",
        ghost: "border-transparent [a&]:hover:bg-accent",
        link: "border-transparent text-foreground underline-offset-4 [a&]:hover:underline",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "span"

  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }
