import { ChevronLeft, ChevronRight } from "lucide-react";
import { IconButton } from "./IconButton";

export interface PaginationProps {
  total: number;
  page: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  className?: string;
}

export function Pagination({
  total,
  page,
  pageSize,
  onPageChange,
  className,
}: PaginationProps) {
  const pageCount = Math.max(1, Math.ceil(total / pageSize));
  const start = total === 0 ? 0 : page * pageSize + 1;
  const end = Math.min(total, (page + 1) * pageSize);

  return (
    <div
      className={`flex items-center justify-between gap-3 border-t border-border px-4 py-3 text-sm text-fg-muted ${className ?? ""}`}
    >
      <span className="tabular-nums">
        {total === 0
          ? "0 results"
          : `${start}–${end} of ${total}`}
      </span>
      <div className="flex items-center gap-2">
        <IconButton
          label="Previous page"
          variant="outline"
          size="sm"
          disabled={page === 0}
          onClick={() => onPageChange(Math.max(0, page - 1))}
        >
          <ChevronLeft className="h-4 w-4" />
        </IconButton>
        <span className="tabular-nums text-xs">
          Page {page + 1} of {pageCount}
        </span>
        <IconButton
          label="Next page"
          variant="outline"
          size="sm"
          disabled={page + 1 >= pageCount}
          onClick={() => onPageChange(Math.min(pageCount - 1, page + 1))}
        >
          <ChevronRight className="h-4 w-4" />
        </IconButton>
      </div>
    </div>
  );
}
