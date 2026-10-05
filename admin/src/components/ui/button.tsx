import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-md text-[13px] font-medium transition-[background-color,border-color,color,box-shadow] duration-100 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-3.5 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:ring-offset-1 focus-visible:ring-offset-background aria-invalid:border-destructive cursor-pointer select-none",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-[0_1px_2px_rgb(0_0_0/0.12)] hover:bg-primary/88",
        destructive:
          "bg-destructive text-white shadow-[0_1px_2px_rgb(0_0_0/0.12)] hover:bg-destructive/90",
        outline:
          "border border-border-strong bg-card text-foreground shadow-[0_1px_2px_rgb(0_0_0/0.04)] hover:bg-accent dark:border-white/10 dark:bg-white/[0.04] dark:shadow-[inset_0_1px_0_rgb(255_255_255/0.04)] dark:hover:border-white/15 dark:hover:bg-white/[0.08]",
        secondary:
          "bg-secondary text-secondary-foreground hover:bg-[color-mix(in_oklab,var(--foreground)_10%,transparent)]",
        ghost:
          "text-muted-foreground hover:bg-accent hover:text-foreground",
        link: "text-foreground underline underline-offset-4 decoration-border-strong hover:decoration-foreground",
      },
      size: {
        default: "h-8 px-3 has-[>svg]:px-2.5",
        xs: "h-6 gap-1 rounded-[5px] px-2 text-xs has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-7 gap-1.5 px-2.5 text-[12.5px] has-[>svg]:px-2",
        lg: "h-9 px-4 has-[>svg]:px-3",
        icon: "size-8",
        "icon-xs": "size-6 rounded-[5px] [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-7",
        "icon-lg": "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
