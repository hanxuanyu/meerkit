import React from "react";
import * as ContextMenuPrimitive from "@radix-ui/react-context-menu";
import { cn } from "../../lib/utils";

export const ContextMenu = ContextMenuPrimitive.Root;
export const ContextMenuTrigger = ContextMenuPrimitive.Trigger;

export function ContextMenuContent({ className, ...props }) {
  return <ContextMenuPrimitive.Portal><ContextMenuPrimitive.Content className={cn("z-50 min-w-44 overflow-hidden rounded-md border bg-popover p-1 text-popover-foreground shadow-md data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95", className)} {...props} /></ContextMenuPrimitive.Portal>;
}

export function ContextMenuItem({ className, inset, ...props }) { return <ContextMenuPrimitive.Item className={cn("relative flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50", inset && "pl-8", className)} {...props} />; }
export function ContextMenuSeparator({ className, ...props }) { return <ContextMenuPrimitive.Separator className={cn("-mx-1 my-1 h-px bg-muted", className)} {...props} />; }
