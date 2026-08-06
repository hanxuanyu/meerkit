import React, { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Braces, Check, ChevronDown } from "lucide-react";
import { Input } from "../ui/Input";
import { fieldSelection, normalizeFieldName, parseLiteralValue } from "./conditionFields";

export function JsonPathInput({ value, onChange, placeholder }) {
  return <div className="condition-json-path">
    <Braces size={13} aria-hidden="true" />
    <Input className="condition-json-path-input" value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} aria-label={placeholder} />
  </div>;
}

export function EditableComparison({ rule, operator, field, fieldGroups, fields, canCompareField, onChange }) {
  const containerRef = useRef(null);
  const menuRef = useRef(null);
  const inputRef = useRef(null);
  const listboxID = useId();
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const [menuStyle, setMenuStyle] = useState({});
  const valueSource = canCompareField && rule.value_source === "current" ? "current" : "literal";
  const selectedField = valueSource === "literal" ? null : fields.find((item) => item.source === valueSource && item.name === normalizeFieldName(rule.value_field));
  const selectableGroups = canCompareField ? fieldGroups : [];
  const selectableFields = selectableGroups.flatMap((group) => group.fields);
  const selectedIndex = selectedField ? selectableFields.findIndex((item) => fieldSelection(item) === fieldSelection(selectedField)) : -1;
  const displayValue = selectedField ? `${selectedField.sourceLabel} · ${selectedField.label}` : String(rule.value ?? "");

  useEffect(() => {
    if (!open) return undefined;
    const updatePosition = () => {
      const rect = containerRef.current?.getBoundingClientRect();
      const dialogRect = containerRef.current?.closest("[role=dialog]")?.getBoundingClientRect();
      if (!rect || !dialogRect) return;
      const gap = 4;
      const availableBelow = dialogRect.bottom - rect.bottom - gap;
      const availableAbove = rect.top - dialogRect.top - gap;
      const openAbove = availableBelow < 220 && availableAbove > availableBelow;
      const maxHeight = Math.max(80, Math.min(240, (openAbove ? availableAbove : availableBelow) - 8));
      setMenuStyle(openAbove
        ? { left: rect.left - dialogRect.left, width: rect.width, bottom: dialogRect.bottom - rect.top + gap, maxHeight }
        : { left: rect.left - dialogRect.left, width: rect.width, top: rect.bottom - dialogRect.top + gap, maxHeight });
    };
    const closeOnOutsidePointer = (event) => {
      if (!containerRef.current?.contains(event.target) && !menuRef.current?.contains(event.target)) setOpen(false);
    };
    updatePosition();
    document.addEventListener("pointerdown", closeOnOutsidePointer);
    window.addEventListener("resize", updatePosition);
    window.addEventListener("scroll", updatePosition, true);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      window.removeEventListener("resize", updatePosition);
      window.removeEventListener("scroll", updatePosition, true);
    };
  }, [open]);

  const enterLiteral = (nextValue) => onChange({
    value_source: "literal",
    value_field: "",
    value_path: "",
    value: parseLiteralValue(nextValue, field, operator)
  });
  const chooseField = (nextField) => {
    onChange({ value_source: nextField.source, value_field: nextField.name, value_path: "" });
    setOpen(false);
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      inputRef.current?.select();
    });
  };
  const toggleMenu = () => {
    if (!canCompareField) return;
    setActiveIndex(selectedIndex >= 0 ? selectedIndex : 0);
    setOpen((current) => !current);
  };
  const handleKeyDown = (event) => {
    if (!canCompareField) return;
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const direction = event.key === "ArrowDown" ? 1 : -1;
      setOpen(true);
      setActiveIndex((current) => {
        const start = current >= 0 ? current : selectedIndex >= 0 ? selectedIndex : direction > 0 ? -1 : 0;
        return (start + direction + selectableFields.length) % selectableFields.length;
      });
    } else if (event.key === "Enter" && open && activeIndex >= 0) {
      event.preventDefault();
      chooseField(selectableFields[activeIndex]);
    } else if (event.key === "Escape" && open) {
      event.preventDefault();
      setOpen(false);
    }
  };

  const portalContainer = open ? containerRef.current?.closest("[role=dialog]") : null;
  const menu = portalContainer ? createPortal(
    <div className="condition-combobox-menu" ref={menuRef} id={listboxID} role="listbox" style={menuStyle}>
      {selectableGroups.map((group) => <div className="condition-combobox-group" key={group.key}>
        <div className="condition-combobox-label">{group.label}</div>
        {group.fields.map((item) => {
          const index = selectableFields.findIndex((fieldItem) => fieldSelection(fieldItem) === fieldSelection(item));
          const selected = selectedField && fieldSelection(selectedField) === fieldSelection(item);
          return <button type="button" className="condition-combobox-option" id={`${listboxID}-option-${index}`} data-active={activeIndex === index ? "true" : undefined} aria-selected={selected} role="option" key={fieldSelection(item)} onMouseEnter={() => setActiveIndex(index)} onClick={() => chooseField(item)}>
            <span>{item.label}</span>{selected && <Check size={13} />}
          </button>;
        })}
      </div>)}
    </div>,
    portalContainer
  ) : null;

  return <div className={`condition-comparison${selectedField?.path ? " condition-json-operand" : ""}`}>
    <div className="condition-combobox" ref={containerRef}>
      <input ref={inputRef} className="condition-combobox-input" value={displayValue} inputMode={["number", "integer"].includes(field?.type) && operator !== "between" ? "decimal" : undefined} placeholder={operator === "between" ? "下限,上限" : "输入固定值或选择结果字段"} aria-label="比较值" aria-autocomplete="list" aria-activedescendant={open && activeIndex >= 0 ? `${listboxID}-option-${activeIndex}` : undefined} aria-controls={canCompareField ? listboxID : undefined} aria-expanded={canCompareField ? open : undefined} role={canCompareField ? "combobox" : undefined} onChange={(event) => enterLiteral(event.target.value)} onFocus={(event) => selectedField && event.currentTarget.select()} onKeyDown={handleKeyDown} />
      {canCompareField && <button type="button" className="condition-combobox-toggle" title="选择结果字段" aria-label="选择结果字段" tabIndex={-1} onMouseDown={(event) => event.preventDefault()} onClick={toggleMenu}><ChevronDown size={14} /></button>}
    </div>
    {selectedField?.path && <JsonPathInput value={rule.value_path || ""} onChange={(value_path) => onChange({ value_path })} placeholder="比较值 JSON 路径" />}
    {menu}
  </div>;
}
