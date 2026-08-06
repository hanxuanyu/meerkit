import React, { useEffect, useState } from "react";
import { Clock3, Trash2 } from "lucide-react";
import { cronPresets } from "../../lib/constants";
import { previewSchedule, schedulePreviewLabel, schedulePreviewTitle } from "../../lib/schedules";
import { IconButton } from "../../components/ui/IconButton";
import { Input } from "../../components/ui/Input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../../components/ui/Select";

export function CronScheduleRow({ schedule, index, removable, onChange, onRemove }) {
  const [preview, setPreview] = useState(null);
  const [validation, setValidation] = useState({ state: "loading", message: "正在校验..." });

  useEffect(() => {
    const expression = schedule.trim();
    if (!expression) {
      setPreview(null);
      setValidation({ state: "error", message: "请输入 Cron 表达式" });
      return undefined;
    }
    let cancelled = false;
    setValidation({ state: "loading", message: "正在校验..." });
    const timer = window.setTimeout(() => {
      previewSchedule(expression).then((result) => {
        if (!cancelled) {
          setPreview(result);
          setValidation({ state: "success", message: schedulePreviewLabel(result) });
        }
      }).catch((error) => {
        if (!cancelled) {
          setPreview(null);
          setValidation({ state: "error", message: `格式错误：${error.message}` });
        }
      });
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [schedule]);

  return <div className="schedule-row">
    <div className="schedule-expression"><Input required aria-invalid={validation.state === "error"} aria-describedby={`cron-feedback-${index}`} aria-label={`Cron 表达式 ${index + 1}`} value={schedule} onChange={(event) => onChange(event.target.value)} placeholder="*/5 * * * *" /><small id={`cron-feedback-${index}`} className={`schedule-feedback is-${validation.state}`} title={preview ? schedulePreviewTitle(preview) : validation.message}>{validation.message}</small></div>
    <Select value="" onValueChange={onChange}><SelectTrigger className="schedule-preset-button" aria-label={`选择 Cron 预设 ${index + 1}`} title="选择 Cron 预设"><SelectValue placeholder={<span><Clock3 size={13} />预设</span>} /></SelectTrigger><SelectContent className="schedule-preset-content">{cronPresets.map((preset) => <SelectItem className="schedule-preset-item" key={preset.value} value={preset.value}>{preset.label}<code>{preset.value}</code></SelectItem>)}</SelectContent></Select>
    <IconButton className="schedule-remove" size="sm" title="删除表达式" aria-label={`删除第 ${index + 1} 个表达式`} disabled={!removable} onClick={onRemove}><Trash2 size={14} /></IconButton>
  </div>;
}
