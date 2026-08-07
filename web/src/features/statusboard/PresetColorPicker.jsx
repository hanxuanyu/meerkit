import React, { useState } from "react";
import { Check } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "../../components/ui/Popover";
import { statusColorPreset, statusColorPresets } from "./statusColorPresets";

export function PresetColorPicker({ color, level, onChange, ariaLabel = "选择状态颜色", disabledColorIDs = [] }) {
  const [open, setOpen] = useState(false);
  const selected = statusColorPreset(color, level);
  const disabledColors = new Set(disabledColorIDs);
  return <Popover modal open={open} onOpenChange={setOpen}>
    <PopoverTrigger asChild><button type="button" className={`preset-color-button color-${selected.id}`} aria-label={`${ariaLabel}，当前${selected.name}`} title={ariaLabel} onClick={(event) => event.stopPropagation()}><span className="sr-only">{selected.name}</span></button></PopoverTrigger>
    <PopoverContent className="preset-color-popover" side="top" align="center" onPointerDown={(event) => event.stopPropagation()} onClick={(event) => event.stopPropagation()}>{statusColorPresets.map((preset) => { const disabled = disabledColors.has(preset.id); return <button type="button" className={`preset-color-dot color-${preset.id}${preset.id === selected.id ? " is-selected" : ""}`} key={preset.id} aria-label={disabled ? `${preset.name}，已被其他区间使用` : preset.name} aria-pressed={preset.id === selected.id} title={disabled ? "该颜色已被其他区间使用" : preset.name} disabled={disabled} onPointerDown={(event) => event.stopPropagation()} onClick={(event) => { event.preventDefault(); event.stopPropagation(); onChange(preset); setOpen(false); }}>{preset.id === selected.id && <Check size={12} />}</button>; })}</PopoverContent>
  </Popover>;
}
