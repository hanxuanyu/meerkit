import React from "react";
import { cva } from "class-variance-authority";
import { cn } from "../../lib/utils";

const iconButtonVariants = cva(
  "icon-button inline-flex shrink-0 items-center justify-center rounded-md border border-transparent text-muted-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        ghost: "bg-transparent hover:bg-accent hover:text-accent-foreground",
        outline: "border-border bg-background hover:bg-accent hover:text-accent-foreground"
      },
      size: {
        sm: "size-7",
        default: "size-8"
      }
    },
    defaultVariants: { variant: "ghost", size: "sm" }
  }
);

export const IconButton = React.forwardRef(function IconButton({ children, className, variant, size, type = "button", title, "aria-label": ariaLabel, ...props }, ref) {
  const hint = title || ariaLabel;
  return <button ref={ref} type={type} title={hint} aria-label={ariaLabel || title} className={cn(iconButtonVariants({ variant, size, className }))} {...props}>{children}</button>;
});
