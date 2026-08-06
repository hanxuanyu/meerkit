import React from "react";
import { cn } from "../../lib/utils";

export function PageHeader({ eyebrow, title, description, actions, className }) {
  return <div className={cn("page-heading", className)}>
    <div><div className="eyebrow">{eyebrow}</div><h1>{title}</h1><p>{description}</p></div>
    {actions}
  </div>;
}
