import React, { useEffect, useId, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Braces, Check, ChevronDown, Trash2 } from "lucide-react";
import { operators } from "../../lib/constants";
import { getResultFieldGroups } from "../../lib/resultSchema";
import { IconButton } from "../ui/IconButton";
import { Input } from "../ui/Input";
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from "../ui/Select";

const fieldReferenceOperators = new Set(["equals", "not_equals", "gt", "gte", "lt", "lte", "contains", "not_contains"]);
const valueOperatorsWithoutInput = new Set(["changed", "is_true", "is_false", "exists", "not_exists"]);
const previousExecutionSelection = "__previous_execution__";

export function ConditionEditor({ descriptor, value, onChange }) {
  const fieldGroups = getResultFieldGroups(descriptor, { includePrevious: false });
  const fields = fieldGroups.flatMap((group) => group.fields);
  const rules = value.rules || [];
  const notificationPolicy = value.notification_policy === "every" ? "every" : "once";
  const updateRule = (index, patch) => onChange({ ...value, rules: rules.map((rule, itemIndex) => itemIndex === index ? { ...rule, ...patch } : rule) });
  const removeRule = (index) => onChange({ ...value, rules: rules.filter((_, itemIndex) => itemIndex !== index) });

  return <>
    <div className="condition-toolbar"><span>满足</span><Select value={value.logic || "ALL"} onValueChange={(logic) => onChange({ ...value, logic })}><SelectTrigger className="condition-select"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="ALL">全部条件</SelectItem><SelectItem value="ANY">任一条件</SelectItem></SelectContent></Select><span>时触发通知</span><span>通知策略</span><Select value={notificationPolicy} onValueChange={(notification_policy) => onChange({ ...value, notification_policy })}><SelectTrigger className="condition-policy-select"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="once">同一场景仅通知一次</SelectItem><SelectItem value="every">每次满足条件都通知</SelectItem></SelectContent></Select></div>
    <div className="condition-list">{rules.length ? rules.map((rule, index) => <ConditionRow key={`${rule.source || "current"}-${rule.field}-${index}`} rule={rule} fieldGroups={fieldGroups} fields={fields} onChange={(patch) => updateRule(index, patch)} onRemove={() => removeRule(index)} />) : <div className="condition-empty">点击右上角 + 添加触发条件</div>}</div>
  </>;
}

function normalizeFieldName(name) {
  return String(name || "").replace(/^(result|current|previous)\./, "");
}

function fieldSelection(field) {
  return field ? `${field.source}|${field.name}` : "";
}

function parseFieldSelection(selection, fields) {
  const separator = selection.indexOf("|");
  const source = separator > 0 ? selection.slice(0, separator) : "current";
  const name = normalizeFieldName(separator > 0 ? selection.slice(separator + 1) : selection);
  return fields.find((field) => field.source === source && field.name === name) || fields[0];
}

function findRuleField(rule, fields) {
  const name = normalizeFieldName(rule.field);
  return fields.find((field) => field.source === "current" && field.name === name) || fields.find((field) => field.source === "current") || fields[0];
}

function executionComparisonOperators(field) {
  if (["number", "integer"].includes(field?.type)) return ["equals", "not_equals", "gt", "gte", "lt", "lte"];
  if (["string", "text"].includes(field?.type)) return ["equals", "not_equals", "contains", "not_contains"];
  return ["equals", "not_equals"];
}

function parseLiteralValue(value, field, operator) {
  if (!["number", "integer"].includes(field?.type) || operator === "between" || value.trim() === "") return value;
  const number = Number(value);
  return Number.isFinite(number) ? number : value;
}

function JsonPathInput({ value, onChange, placeholder }) {
  return <div className="condition-json-path">
    <Braces size={13} aria-hidden="true" />
    <Input className="condition-json-path-input" value={value} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} aria-label={placeholder} />
  </div>;
}

function EditableComparison({ rule, operator, field, fieldGroups, fields, canCompareField, onChange }) {
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
          return <button
            type="button"
            className="condition-combobox-option"
            id={`${listboxID}-option-${index}`}
            data-active={activeIndex === index ? "true" : undefined}
            aria-selected={selected}
            role="option"
            key={fieldSelection(item)}
            onMouseEnter={() => setActiveIndex(index)}
            onClick={() => chooseField(item)}
          >
            <span>{item.label}</span>
            {selected && <Check size={13} />}
          </button>;
        })}
      </div>)}
    </div>,
    portalContainer
  ) : null;

  return <div className={`condition-comparison${selectedField?.path ? " condition-json-operand" : ""}`}>
    <div className="condition-combobox" ref={containerRef}>
      <input
        ref={inputRef}
        className="condition-combobox-input"
        value={displayValue}
        inputMode={["number", "integer"].includes(field?.type) && operator !== "between" ? "decimal" : undefined}
        placeholder={operator === "between" ? "下限,上限" : "输入固定值或选择结果字段"}
        aria-label="比较值"
        aria-autocomplete="list"
        aria-activedescendant={open && activeIndex >= 0 ? `${listboxID}-option-${activeIndex}` : undefined}
        aria-controls={canCompareField ? listboxID : undefined}
        aria-expanded={canCompareField ? open : undefined}
        role={canCompareField ? "combobox" : undefined}
        onChange={(event) => enterLiteral(event.target.value)}
        onFocus={(event) => selectedField && event.currentTarget.select()}
        onKeyDown={handleKeyDown}
      />
      {canCompareField && <button type="button" className="condition-combobox-toggle" title="选择结果字段" aria-label="选择结果字段" tabIndex={-1} onMouseDown={(event) => event.preventDefault()} onClick={toggleMenu}><ChevronDown size={14} /></button>}
    </div>
    {selectedField?.path && <JsonPathInput value={rule.value_path || ""} onChange={(value_path) => onChange({ value_path })} placeholder="比较值 JSON 路径" />}
    {menu}
  </div>;
}

function ConditionRow({ rule, fieldGroups, fields, onChange, onRemove }) {
  const field = findRuleField(rule, fields);
  const comparesExecutions = rule.source === "previous" && rule.value_source === "current";
  const operatorsForField = comparesExecutions ? executionComparisonOperators(field) : field?.operators || ["equals"];
  const operator = operatorsForField.includes(rule.operator) ? rule.operator : operatorsForField[0] || "equals";
  const showValue = !valueOperatorsWithoutInput.has(operator);
  const canCompareField = showValue && fieldReferenceOperators.has(operator);

  const changeField = (selection) => {
    if (selection === previousExecutionSelection) {
      const comparisonOperators = executionComparisonOperators(field);
      const nextOperator = comparisonOperators.includes(operator) ? operator : comparisonOperators[0];
      onChange({ field: field?.name || "", source: "previous", path: rule.path || "", operator: nextOperator, value_source: "current", value_field: field?.name || "", value_path: rule.path || "", value: "" });
      return;
    }
    const nextField = parseFieldSelection(selection, fields);
    onChange({ field: nextField?.name || "", source: nextField?.source || "current", path: "", operator: nextField?.operators?.[0] || "equals", value_source: "literal", value_field: "", value_path: "", value: "" });
  };
  const changeOperator = (nextOperator) => comparesExecutions
    ? onChange({ operator: nextOperator, value_source: "current", value_field: field?.name || "", value_path: rule.path || "" })
    : onChange({ operator: nextOperator, value_source: fieldReferenceOperators.has(nextOperator) ? (rule.value_source || "literal") : "literal", value_field: fieldReferenceOperators.has(nextOperator) ? rule.value_field : "", value_path: fieldReferenceOperators.has(nextOperator) ? rule.value_path : "" });
  const changeExecutionField = (selection) => {
    const nextField = parseFieldSelection(selection, fields);
    const comparisonOperators = executionComparisonOperators(nextField);
    onChange({
      field: nextField?.name || "",
      source: "previous",
      path: "",
      operator: comparisonOperators.includes(operator) ? operator : comparisonOperators[0],
      value_source: "current",
      value_field: nextField?.name || "",
      value_path: "",
      value: ""
    });
  };
  const changeExecutionPath = (path) => onChange({ path, value_path: path });

  return <div className="condition-row">
    <div className={`condition-left${!comparesExecutions && field?.path ? " condition-json-operand" : ""}`}>
      <Select value={comparesExecutions ? previousExecutionSelection : fieldSelection(field)} onValueChange={changeField}><SelectTrigger><SelectValue placeholder="选择结果字段" /></SelectTrigger><SelectContent><SelectGroup><SelectLabel>执行对比</SelectLabel><SelectItem value={previousExecutionSelection}>上次执行</SelectItem></SelectGroup>{fieldGroups.map((group) => <SelectGroup key={group.key}><SelectLabel>{group.label}</SelectLabel>{group.fields.map((item) => <SelectItem key={fieldSelection(item)} value={fieldSelection(item)}>{item.label}</SelectItem>)}</SelectGroup>)}</SelectContent></Select>
      {!comparesExecutions && field?.path && <JsonPathInput value={rule.path || ""} onChange={(path) => onChange({ path })} placeholder="JSON 路径，如 data.status" />}
    </div>
    <Select value={operator} onValueChange={changeOperator}><SelectTrigger><SelectValue /></SelectTrigger><SelectContent>{operatorsForField.map((item) => <SelectItem key={item} value={item}>{operators[item] || item}</SelectItem>)}</SelectContent></Select>
    {comparesExecutions ? <div className={`condition-comparison${field?.path ? " condition-json-operand" : ""}`}><Select value={fieldSelection(field)} onValueChange={changeExecutionField}><SelectTrigger aria-label="选择当前执行结果字段"><span>当前执行 · {field?.label || "选择结果字段"}</span></SelectTrigger><SelectContent>{fieldGroups.map((group) => <SelectGroup key={group.key}><SelectLabel>{group.label}</SelectLabel>{group.fields.map((item) => <SelectItem key={fieldSelection(item)} value={fieldSelection(item)}>{item.label}</SelectItem>)}</SelectGroup>)}</SelectContent></Select>{field?.path && <JsonPathInput value={rule.path || ""} onChange={changeExecutionPath} placeholder="当前与上次 JSON 路径" />}</div> : showValue ? <EditableComparison rule={rule} operator={operator} field={field} fieldGroups={fieldGroups} fields={fields} canCompareField={canCompareField} onChange={onChange} /> : <span className="condition-value-placeholder" aria-hidden="true" />}
    <IconButton size="sm" title="删除条件" aria-label="删除条件" onClick={onRemove}><Trash2 size={14} /></IconButton>
  </div>;
}
