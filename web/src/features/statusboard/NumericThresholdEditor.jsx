import React, { useEffect, useMemo, useState } from "react";
import { AlignHorizontalDistributeCenter, ArrowUpDown, Palette, Plus, Trash2 } from "lucide-react";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Slider } from "../../components/ui/Slider";
import { Checkbox } from "../../components/ui/Checkbox";
import { PresetColorPicker } from "./PresetColorPicker";
import { statusColorPreset, statusColorPresets } from "./statusColorPresets";

const maximumSegmentCount = 6;

export function NumericThresholdEditor({ thresholds, onChange }) {
  const [initialMaximum] = useState(() => recommendedScaleMaximum(thresholds));
  const [scaleMaximum, setScaleMaximum] = useState(initialMaximum);
  const [scaleDraft, setScaleDraft] = useState(formatNumber(initialMaximum));
  const [reverseColorOrder, setReverseColorOrder] = useState(false);
  const finiteThresholds = thresholds.slice(0, -1);
  const scaleMinimum = recommendedScaleMinimum(thresholds);
  const largestBoundary = finiteThresholds.at(-1)?.maximum ?? 0;
  const step = scaleStep(scaleMaximum - scaleMinimum);
  const values = finiteThresholds.map((threshold) => Number(threshold.maximum));

  useEffect(() => {
    if (largestBoundary < scaleMaximum) return;
    const next = recommendedScaleMaximum(thresholds);
    setScaleMaximum(next);
    setScaleDraft(formatNumber(next));
  }, [largestBoundary, scaleMaximum, thresholds]);

  const trackBackground = useMemo(() => thresholdGradient(thresholds, scaleMinimum, scaleMaximum), [scaleMaximum, scaleMinimum, thresholds]);

  const commitScaleMaximum = () => {
    const parsed = Number(scaleDraft);
    if (Number.isFinite(parsed) && parsed > Math.max(0, largestBoundary)) {
      setScaleMaximum(parsed);
      setScaleDraft(formatNumber(parsed));
      return;
    }
    setScaleDraft(formatNumber(scaleMaximum));
  };

  const updateThreshold = (index, patch) => onChange(thresholds.map((threshold, itemIndex) => itemIndex === index ? { ...threshold, ...patch } : threshold));
  const updateBoundaries = (nextValues) => onChange(thresholds.map((threshold, index) => index < nextValues.length ? { ...threshold, maximum: nextValues[index] } : threshold));
  const updateBoundary = (index, requested) => {
    if (!Number.isFinite(requested)) return;
    const previous = index > 0 ? Number(thresholds[index - 1].maximum) : Number.NEGATIVE_INFINITY;
    const following = thresholds[index + 1]?.maximum;
    const upper = following == null ? Number.POSITIVE_INFINITY : Number(following);
    const minimumAllowed = Number.isFinite(previous) ? previous + step : requested;
    const maximumAllowed = Number.isFinite(upper) ? upper - step : requested;
    updateThreshold(index, { maximum: roundToStep(clamp(requested, minimumAllowed, maximumAllowed), step) });
  };
  const addBoundary = () => {
    if (thresholds.length >= maximumSegmentCount) return;
    const positions = [scaleMinimum, ...values.map((value) => clamp(value, scaleMinimum, scaleMaximum)), scaleMaximum];
    let widestIndex = 0;
    for (let index = 1; index < positions.length - 1; index += 1) {
      if (positions[index + 1] - positions[index] >= positions[widestIndex + 1] - positions[widestIndex]) widestIndex = index;
    }
    const lower = positions[widestIndex];
    const upper = positions[widestIndex + 1];
    if (upper - lower <= step * 2) return;
    const maximum = roundToStep((lower + upper) / 2, step);
    const owner = thresholds[widestIndex];
    const newColor = nearestUnusedColor(thresholds, owner, reverseColorOrder);
    const result = [...thresholds];
    result.splice(widestIndex, 0, { ...owner, maximum });
    result[widestIndex + 1] = { ...owner, color: newColor };
    onChange(result);
  };
  const removeBoundary = (index) => onChange(thresholds.filter((_, itemIndex) => itemIndex !== index));
  const sortColors = () => {
    const colors = distributedColors(thresholds.length, reverseColorOrder);
    onChange(thresholds.map((threshold, index) => ({ ...threshold, color: colors[index] })));
  };
  const reverseColors = () => {
    const colors = thresholds.map((threshold) => statusColorPreset(threshold.color, threshold.level).id).reverse();
    setReverseColorOrder((current) => !current);
    onChange(thresholds.map((threshold, index) => ({ ...threshold, color: colors[index] })));
  };
  const distributeBoundaries = () => {
    if (thresholds.length < 2) return;
    const span = scaleMaximum - scaleMinimum;
    const nextValues = Array.from(
      { length: thresholds.length - 1 },
      (_, index) => roundToStep(scaleMinimum + span * ((index + 1) / thresholds.length), step)
    );
    updateBoundaries(nextValues);
  };

  return <div className="numeric-threshold-editor">
    <div className="threshold-editor-toolbar"><label className="threshold-scale-field"><span>可视上限</span><Input type="number" min="0" step="any" value={scaleDraft} onChange={(event) => { setScaleDraft(event.target.value); const value = Number(event.target.value); if (Number.isFinite(value) && value > Math.max(0, largestBoundary)) setScaleMaximum(value); }} onBlur={commitScaleMaximum} onKeyDown={(event) => { if (event.key === "Enter") { event.preventDefault(); commitScaleMaximum(); event.currentTarget.blur(); } }} /></label><div className="threshold-editor-actions" role="toolbar" aria-label="数值区间工具栏"><IconButton variant="ghost" size="sm" title={reverseColorOrder ? "按分段数从红到绿生成颜色" : "按分段数从绿到红生成颜色"} aria-label={reverseColorOrder ? "按分段数从红到绿生成颜色" : "按分段数从绿到红生成颜色"} disabled={thresholds.length < 2} onClick={sortColors}><Palette size={14} /></IconButton><IconButton className={reverseColorOrder ? "is-active" : ""} variant="ghost" size="sm" title="反转当前区间颜色" aria-label="反转当前区间颜色" aria-pressed={reverseColorOrder} disabled={thresholds.length < 2} onClick={reverseColors}><ArrowUpDown size={14} /></IconButton><IconButton variant="ghost" size="sm" title="均匀分布区间游标" aria-label="均匀分布区间游标" disabled={thresholds.length < 2} onClick={distributeBoundaries}><AlignHorizontalDistributeCenter size={14} /></IconButton><IconButton variant="ghost" size="sm" title={thresholds.length >= maximumSegmentCount ? "最多支持 6 个区间" : "添加区间分界"} aria-label={thresholds.length >= maximumSegmentCount ? "已达到 6 个区间上限" : "添加区间分界"} disabled={thresholds.length >= maximumSegmentCount} onClick={addBoundary}><Plus size={14} /></IconButton></div></div>
    <div className="threshold-slider-panel">
      <div className="threshold-cursor-layer">{finiteThresholds.map((threshold, index) => { const percent = ((Number(threshold.maximum) - scaleMinimum) / (scaleMaximum - scaleMinimum)) * 100; return <label className="threshold-cursor-badge" key={index} style={{ left: `clamp(34px, ${percent}%, calc(100% - 34px))` }}><Input type="number" step="any" value={threshold.maximum ?? ""} onChange={(event) => updateBoundary(index, Number(event.target.value))} aria-label={`第 ${index + 1} 个区间游标精确值`} /></label>; })}</div>
      <Slider className="threshold-slider" min={scaleMinimum} max={scaleMaximum} step={step} minStepsBetweenThumbs={1} value={values} onValueChange={updateBoundaries} trackStyle={{ background: trackBackground }} />
      <div className="threshold-scale-axis"><span>{formatNumber(scaleMinimum)}</span><span>{formatNumber(scaleMaximum)}</span></div>
    </div>
    <div className="threshold-interval-list">{thresholds.map((threshold, index) => <div className="threshold-interval-row" key={index}><strong>{intervalLabel(thresholds, index)}</strong><label className="threshold-abnormal-toggle"><Checkbox checked={threshold.level !== "success"} onCheckedChange={(checked) => updateThreshold(index, { level: checked ? "failure" : "success", label: checked ? "异常" : "正常" })} /><span>异常</span></label><PresetColorPicker color={threshold.color} level={threshold.level} disabledColorIDs={thresholds.filter((_, itemIndex) => itemIndex !== index).map((item) => statusColorPreset(item.color, item.level).id)} ariaLabel={`第 ${index + 1} 个区间颜色`} onChange={(preset) => updateThreshold(index, { color: preset.id })} />{index < thresholds.length - 1 ? <IconButton size="sm" title="删除区间分界" aria-label="删除区间分界" onClick={() => removeBoundary(index)}><Trash2 size={13} /></IconButton> : <span className="threshold-row-spacer" />}</div>)}</div>
  </div>;
}

function intervalLabel(thresholds, index) {
  const previous = index > 0 ? thresholds[index - 1].maximum : null;
  const current = thresholds[index].maximum;
  if (previous == null && current == null) return "全部数值";
  if (previous == null) return `≤ ${formatNumber(current)}`;
  if (current == null) return `> ${formatNumber(previous)}`;
  return `> ${formatNumber(previous)} 且 ≤ ${formatNumber(current)}`;
}

function thresholdGradient(thresholds, minimum, maximum) {
  const span = maximum - minimum;
  let start = 0;
  const stops = [];
  for (const threshold of thresholds) {
    const end = threshold.maximum == null ? 100 : clamp(((Number(threshold.maximum) - minimum) / span) * 100, 0, 100);
    const color = statusColorPreset(threshold.color, threshold.level).color;
    stops.push(`${color} ${start}%`, `${color} ${end}%`);
    start = end;
  }
  return `linear-gradient(to right, ${stops.join(", ")})`;
}

function distributedColors(count, reverse) {
  if (count <= 0) return [];
  if (count === 1) return [statusColorPresets[reverse ? statusColorPresets.length - 1 : 0].id];
  const lastIndex = statusColorPresets.length - 1;
  const colors = Array.from({ length: count }, (_, index) => statusColorPresets[Math.round((index * lastIndex) / (count - 1))].id);
  return reverse ? colors.reverse() : colors;
}

function nearestUnusedColor(thresholds, owner, reverse) {
  const used = new Set(thresholds.map((threshold) => statusColorPreset(threshold.color, threshold.level).id));
  const ownerIndex = statusColorPresets.findIndex((preset) => preset.id === statusColorPreset(owner.color, owner.level).id);
  const candidates = statusColorPresets
    .map((preset, index) => ({ ...preset, index }))
    .filter((preset) => !used.has(preset.id))
    .sort((left, right) => Math.abs(left.index - ownerIndex) - Math.abs(right.index - ownerIndex) || (reverse ? left.index - right.index : right.index - left.index));
  return candidates[0]?.id || distributedColors(thresholds.length + 1, reverse).at(-1);
}

function recommendedScaleMaximum(thresholds) {
  const largest = Math.max(0, ...thresholds.filter((threshold) => threshold.maximum != null).map((threshold) => Number(threshold.maximum) || 0));
  if (largest <= 0) return 100;
  return niceCeiling(largest * 1.25);
}

function recommendedScaleMinimum(thresholds) {
  const smallest = Math.min(0, ...thresholds.filter((threshold) => threshold.maximum != null).map((threshold) => Number(threshold.maximum) || 0));
  return smallest < 0 ? -niceCeiling(Math.abs(smallest) * 1.25) : 0;
}

function niceCeiling(value) {
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const normalized = value / magnitude;
  return ([1, 2, 2.5, 5, 10].find((candidate) => candidate >= normalized) || 10) * magnitude;
}

function scaleStep(span) {
  if (!Number.isFinite(span) || span <= 0) return 1;
  return 10 ** (Math.floor(Math.log10(span)) - 2);
}

function roundToStep(value, step) {
  const precision = Math.max(0, -Math.floor(Math.log10(step)));
  return Number(value.toFixed(Math.min(precision + 1, 10)));
}

function clamp(value, minimum, maximum) { return Math.min(Math.max(value, minimum), maximum); }
function formatNumber(value) { return Number(value).toLocaleString("zh-CN", { maximumFractionDigits: 8, useGrouping: false }); }
