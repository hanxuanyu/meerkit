import React, { useState } from "react";
import { Ellipsis } from "lucide-react";
import { IconButton } from "./IconButton";
import { Popover, PopoverContent, PopoverTrigger } from "./Popover";

export function ActionMenu({ items, label = "更多操作", align = "end", triggerVariant = "ghost" }) {
  const [open, setOpen] = useState(false);
  const visibleItems = (items || []).filter(Boolean);

  return <Popover open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild><IconButton className="action-menu-trigger" variant={triggerVariant} size="sm" title={label} aria-label={label}><Ellipsis size={16} /></IconButton></PopoverTrigger>
    <PopoverContent className="action-menu-content" align={align} sideOffset={5} role="menu" aria-label={label}>
      {visibleItems.map((item) => {
        const Icon = item.icon;
        return <button key={item.label} type="button" role="menuitem" className={`action-menu-item${item.destructive ? " is-destructive" : ""}`} disabled={item.disabled} onClick={() => { setOpen(false); item.onSelect?.(); }}>
          {Icon && <Icon size={14} />}
          <span>{item.label}</span>
        </button>;
      })}
    </PopoverContent>
  </Popover>;
}
