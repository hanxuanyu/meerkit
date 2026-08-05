import React from "react";
import { cva } from "class-variance-authority";
import { cn } from "../../lib/utils";

const badgeVariants = cva("badge inline-flex items-center rounded-md border px-2.5 py-0.5 text-xs font-semibold transition-colors", {
  variants: { variant: { default: "border-transparent bg-primary text-primary-foreground", secondary: "border-transparent bg-secondary text-secondary-foreground", outline: "text-foreground" } },
  defaultVariants: { variant: "default" }
});

export function Badge({ children, tone = "neutral", variant, className = "" }) {
  return <span className={cn(badgeVariants({ variant: variant || (tone === "neutral" ? "outline" : "default") }), `badge-${tone}`, className)}>{children}</span>;
}
