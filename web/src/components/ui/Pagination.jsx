import React from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { IconButton } from "./IconButton";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "./Select";

export function Pagination({ page, pageSize, total, onPageChange, onPageSizeChange, disabled = false }) {
  if (!total) return null;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(Math.max(page, 1), totalPages);
  const start = (currentPage - 1) * pageSize + 1;
  const end = Math.min(currentPage * pageSize, total);

  return <div className="pagination">
    <span className="pagination-summary">显示 {start}-{end}，共 {total} 项</span>
    <div className="pagination-controls">
      <Select value={String(pageSize)} onValueChange={(value) => onPageSizeChange(Number(value))} disabled={disabled}>
        <SelectTrigger className="pagination-size" aria-label="每页数量"><SelectValue /></SelectTrigger>
        <SelectContent><SelectItem value="10">每页 10 条</SelectItem><SelectItem value="20">每页 20 条</SelectItem><SelectItem value="50">每页 50 条</SelectItem><SelectItem value="100">每页 100 条</SelectItem></SelectContent>
      </Select>
      <span className="pagination-page">第 {currentPage} / {totalPages} 页</span>
      <IconButton variant="outline" size="sm" title="上一页" aria-label="上一页" disabled={disabled || currentPage <= 1} onClick={() => onPageChange(currentPage - 1)}><ChevronLeft size={14} /></IconButton>
      <IconButton variant="outline" size="sm" title="下一页" aria-label="下一页" disabled={disabled || currentPage >= totalPages} onClick={() => onPageChange(currentPage + 1)}><ChevronRight size={14} /></IconButton>
    </div>
  </div>;
}
