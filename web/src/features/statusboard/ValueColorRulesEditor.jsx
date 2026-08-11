import React from "react";
import { Plus, Trash2 } from "lucide-react";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Checkbox } from "../../components/ui/Checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";
import { PresetColorPicker } from "./PresetColorPicker";
import { statusColorPreset } from "./statusColorPresets";

export function ValueColorRulesEditor({ type, source, onChange }) {
  const mappings = source.value_mappings || [];
  const updateSource = (patch) => onChange({ ...source, ...patch });
  const updateMapping = (index, patch) => updateSource({ value_mappings: mappings.map((mapping, itemIndex) => itemIndex === index ? { ...mapping, ...patch } : mapping) });
  const addMapping = () => updateSource({ value_mappings: [...mappings, { value: type === "number" ? nextNumericValue(mappings) : `值${mappings.length + 1}`, match_type: "exact", level: "failure", label: "失败", color: "red" }] });
  const removeMapping = (index) => updateSource({ value_mappings: mappings.filter((_, itemIndex) => itemIndex !== index) });

  const defaultPreset = statusColorPreset(source.default_color, source.default_level || "success");
  return <div className="value-color-editor">
    <div className="value-color-heading"><div><strong>{type === "number" ? "精确值覆盖" : "文本匹配颜色"}</strong><span>{type === "number" ? "命中后优先于数值区间" : "从上到下首个命中生效，未命中时使用默认颜色"}</span></div><IconButton className="section-add" size="sm" title="添加特定值颜色" aria-label="添加特定值颜色" onClick={addMapping}><Plus size={14} /></IconButton></div>
    {type === "text" && <div className="value-color-default"><strong>其他文本</strong><label className="threshold-abnormal-toggle"><Checkbox checked={(source.default_level || "success") !== "success"} onCheckedChange={(checked) => updateSource({ default_level: checked ? "failure" : "success", default_label: checked ? "异常" : "正常" })} /><span>异常</span></label><PresetColorPicker color={defaultPreset.id} level={source.default_level} ariaLabel="文本默认颜色" onChange={(preset) => updateSource({ default_color: preset.id })} /></div>}
    <div className="value-color-list">{mappings.map((mapping, index) => { const preset = statusColorPreset(mapping.color, mapping.level); const matchType = mapping.match_type || "exact"; return <div className={`value-color-row${type === "text" ? " is-text" : ""}`} key={index}>{type === "text" && <Select value={matchType} onValueChange={(match_type) => updateMapping(index, { match_type })}><SelectTrigger className="value-match-type-select" aria-label={`第 ${index + 1} 条匹配方式`}><SelectValue /></SelectTrigger><SelectContent><SelectItem value="exact">精确</SelectItem><SelectItem value="regex">正则</SelectItem></SelectContent></Select>}<Input type={type === "number" ? "number" : "text"} step={type === "number" ? "any" : undefined} value={mapping.value} onChange={(event) => updateMapping(index, { value: event.target.value })} placeholder={type === "number" ? "精确数值" : matchType === "regex" ? "例如 ^ERROR(_.*)?$" : "精确文本（可为空）"} aria-label={`第 ${index + 1} 条匹配值`} /><label className="threshold-abnormal-toggle"><Checkbox checked={mapping.level !== "success"} onCheckedChange={(checked) => updateMapping(index, { level: checked ? "failure" : "success", label: checked ? "异常" : "正常" })} /><span>异常</span></label><PresetColorPicker color={preset.id} level={mapping.level} ariaLabel={`第 ${index + 1} 条匹配颜色`} onChange={(selected) => updateMapping(index, { color: selected.id })} /><IconButton className="repeat-row-remove" size="sm" title="删除特定值规则" aria-label="删除特定值规则" onClick={() => removeMapping(index)}><Trash2 size={13} /></IconButton></div>; })}</div>
  </div>;
}

function nextNumericValue(mappings) {
  const used = new Set(mappings.map((mapping) => Number(mapping.value)).filter(Number.isFinite));
  let value = 0;
  while (used.has(value)) value += 1;
  return String(value);
}
