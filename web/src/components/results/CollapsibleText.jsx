import React from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

export function CollapsibleText({ value, label = "文本内容", meta = "", expanded = false, onToggle, className = "", collapseLength = 180 }) {
  const text = value === undefined || value === null ? "" : String(value);
  const collapsible = text.split("\n").length > 3 || text.length > collapseLength;
  const contentID = React.useId();
  return <div className={`collapsible-text ${collapsible ? "is-collapsible" : ""} ${collapsible && expanded ? "is-expanded" : ""} ${collapsible && !expanded ? "is-collapsed" : ""} ${className}`}>
    {collapsible ? <button type="button" className="collapsible-text-heading" title={`${expanded ? "收起" : "展开"}${label}`} aria-controls={contentID} aria-expanded={expanded} onClick={onToggle}><span>{label}</span>{meta && <small>{meta}</small>}<span className="collapsible-text-chevron" aria-hidden="true">{expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}</span></button> : <div className="collapsible-text-heading"><span>{label}</span>{meta && <small>{meta}</small>}</div>}
    <pre id={contentID} hidden={collapsible && !expanded}>{text}</pre>
  </div>;
}
