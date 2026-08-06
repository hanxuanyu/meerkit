import React from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import { IconButton } from "../ui/IconButton";

export function CollapsibleText({ value, expanded = false, onToggle, className = "", collapseLength = 180 }) {
  const text = value === undefined || value === null ? "" : String(value);
  const collapsible = text.split("\n").length > 3 || text.length > collapseLength;
  return <div className={`collapsible-text ${collapsible ? "is-collapsible" : ""} ${collapsible && !expanded ? "is-collapsed" : ""} ${className}`}>
    <pre>{text}</pre>
    {collapsible && <IconButton className="collapsible-text-toggle" size="sm" variant="outline" title={expanded ? "收起多行文本" : "展开多行文本"} aria-label={expanded ? "收起多行文本" : "展开多行文本"} aria-expanded={expanded} onClick={onToggle}>{expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}</IconButton>}
  </div>;
}
