import React from "react";
import * as SelectPrimitive from "@radix-ui/react-select";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import { cn } from "../../lib/utils";

export function Select(props) {
  const normalizedProps = Object.prototype.hasOwnProperty.call(props, "value")
    ? { ...props, value: props.value ?? "" }
    : props;
  return <SelectPrimitive.Root {...normalizedProps} />;
}
export const SelectGroup = SelectPrimitive.Group;
export const SelectValue = SelectPrimitive.Value;

export const SelectTrigger = React.forwardRef(function SelectTrigger({ className, children, ...props }, ref) {
  return <SelectPrimitive.Trigger ref={ref} className={cn("select-trigger flex h-9 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-xs shadow-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed disabled:opacity-50", className)} {...props}>{children}<SelectPrimitive.Icon asChild><ChevronDown className="size-4 opacity-50" /></SelectPrimitive.Icon></SelectPrimitive.Trigger>;
});

export const SelectContent = React.forwardRef(function SelectContent({ className, children, position = "popper", ...props }, ref) {
  return <SelectPrimitive.Portal><SelectPrimitive.Content ref={ref} className={cn("relative z-50 max-h-96 min-w-32 overflow-hidden rounded-md border border-border bg-popover text-popover-foreground shadow-md outline-none", position === "popper" && "translate-y-1", className)} position={position} {...props}><SelectScrollUpButton /><SelectPrimitive.Viewport className="p-1">{children}</SelectPrimitive.Viewport><SelectScrollDownButton /></SelectPrimitive.Content></SelectPrimitive.Portal>;
});

export function SelectLabel({ children, className }) { return <SelectPrimitive.Label className={cn("px-2 py-1.5 text-sm font-semibold", className)}>{children}</SelectPrimitive.Label>; }

export const SelectItem = React.forwardRef(function SelectItem({ className, children, ...props }, ref) {
  return <SelectPrimitive.Item ref={ref} className={cn("relative flex w-full cursor-default select-none items-center rounded-sm py-1.5 pl-8 pr-2 text-xs outline-none focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none data-[disabled]:opacity-50", className)} {...props}><span className="absolute left-2 flex size-3.5 items-center justify-center"><SelectPrimitive.ItemIndicator><Check className="size-4" /></SelectPrimitive.ItemIndicator></span><SelectPrimitive.ItemText>{children}</SelectPrimitive.ItemText></SelectPrimitive.Item>;
});

export function SelectSeparator({ className }) { return <SelectPrimitive.Separator className={cn("-mx-1 my-1 h-px bg-muted", className)} />; }
function SelectScrollUpButton({ className, ...props }) { return <SelectPrimitive.ScrollUpButton className={cn("flex cursor-default items-center justify-center py-1", className)} {...props}><ChevronUp className="size-4" /></SelectPrimitive.ScrollUpButton>; }
function SelectScrollDownButton({ className, ...props }) { return <SelectPrimitive.ScrollDownButton className={cn("flex cursor-default items-center justify-center py-1", className)} {...props}><ChevronDown className="size-4" /></SelectPrimitive.ScrollDownButton>; }
