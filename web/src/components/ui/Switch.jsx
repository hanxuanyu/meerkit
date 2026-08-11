import React from "react";
import * as SwitchPrimitive from "@radix-ui/react-switch";
import { cn } from "../../lib/utils";

export const Switch = React.forwardRef(function Switch({ className, ...props }, ref) {
  const { label, description, rowClassName, ...switchProps } = props;
  const control = <SwitchPrimitive.Root ref={ref} className={cn("switch-control", className)} {...switchProps}><SwitchPrimitive.Thumb className="switch-thumb" /></SwitchPrimitive.Root>;
  if (!label && !description) return control;
  return <label className={cn("switch-row", rowClassName)}>{control}<span><strong>{label}</strong>{description && <small>{description}</small>}</span></label>;
});
