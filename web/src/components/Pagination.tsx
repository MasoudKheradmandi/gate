import { ChevronLeft, ChevronRight } from "lucide-react";
import { useI18n } from "../i18n";
import { cn } from "../lib/utils";
import { IconButton } from "./ui/button";
import { Select } from "./ui/form";

export const PAGE_SIZES = [20, 50, 100] as const;

/** Offset pager: page-size select, "1–20 / 134" range text, prev/next. */
export function Pagination({ total, limit, offset, onOffset, onLimit, sizes = PAGE_SIZES, className }: {
  total: number; limit: number; offset: number; onOffset: (o: number) => void; onLimit: (l: number) => void;
  sizes?: readonly number[]; className?: string;
}) {
  const { t } = useI18n();
  const from = total ? offset + 1 : 0;
  const to = Math.min(offset + limit, total);
  const page = Math.floor(offset / limit) + 1;
  const pages = Math.max(1, Math.ceil(total / limit));
  return (
    <nav aria-label={t("pagination")} className={cn("flex flex-wrap items-center gap-3 text-xs text-fg-subtle", className)}>
      <label className="flex items-center gap-2">
        <span>{t("pageSize")}</span>
        <div className="w-20">
          <Select aria-label={t("pageSize")} className="!h-7 text-xs" value={limit} onChange={(e) => { onLimit(Number(e.target.value)); onOffset(0); }}>
            {sizes.map((s) => <option key={s} value={s}>{s}</option>)}
          </Select>
        </div>
      </label>
      <span className="num ml-auto" aria-live="polite">{t("pageRange", { from, to, total })}</span>
      <span className="sr-only">{t("pageNofM", { n: page, m: pages })}</span>
      <div className="flex items-center gap-1">
        <IconButton label={t("prevPage")} size="icon" className="h-7 w-7" disabled={offset <= 0} onClick={() => onOffset(Math.max(0, offset - limit))}><ChevronLeft className="h-4 w-4" /></IconButton>
        <IconButton label={t("nextPage")} size="icon" className="h-7 w-7" disabled={offset + limit >= total} onClick={() => onOffset(offset + limit)}><ChevronRight className="h-4 w-4" /></IconButton>
      </div>
    </nav>
  );
}
