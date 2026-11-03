import { useId, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { Check, X } from "lucide-react";
import { cn } from "../../lib/utils";

export interface PickOption { value: string; label: string; hint?: string; prefix?: ReactNode }

/**
 * Searchable multi-select (ARIA combobox + listbox). Selected values show as removable chips.
 * Arrow keys move, Enter toggles the highlighted option, Escape closes, Backspace on empty removes the last chip.
 */
export function MultiPicker({ value, onChange, options, label, placeholder, invalid, disabled, renderChip, max = 8 }: {
  value: string[]; onChange: (v: string[]) => void; options: PickOption[]; label: string; placeholder?: string;
  invalid?: boolean; disabled?: boolean; renderChip?: (v: string) => ReactNode; max?: number;
}) {
  const [q, setQ] = useState("");
  const [open, setOpen] = useState(false);
  const [hi, setHi] = useState(0);
  const listId = useId();
  const inputRef = useRef<HTMLInputElement>(null);
  const byValue = useMemo(() => new Map(options.map((o) => [o.value, o])), [options]);
  const matches = useMemo(() => {
    const s = q.trim().toLowerCase();
    const list = s ? options.filter((o) => o.value.toLowerCase().includes(s) || o.label.toLowerCase().includes(s) || o.hint?.toLowerCase().includes(s)) : options;
    return list.slice(0, s ? 50 : Math.max(max, 50));
  }, [options, q, max]);
  const toggle = (v: string) => { onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v]); setQ(""); };
  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") { e.preventDefault(); setOpen(true); setHi((h) => Math.min(h + 1, matches.length - 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setHi((h) => Math.max(h - 1, 0)); }
    else if (e.key === "Enter") { e.preventDefault(); if (open && matches[hi]) toggle(matches[hi].value); else setOpen(true); }
    else if (e.key === "Escape") { if (open) { e.preventDefault(); e.stopPropagation(); setOpen(false); } }
    else if (e.key === "Backspace" && !q && value.length) onChange(value.slice(0, -1));
  };
  const optId = (i: number) => `${listId}-o${i}`;
  return (
    <div className="relative">
      <div aria-invalid={invalid || undefined} onClick={() => inputRef.current?.focus()}
        className={cn("flex min-h-9 flex-wrap items-center gap-1.5 rounded-[3px] border border-line border-b-line-strong bg-surface px-2 py-1.5 transition focus-within:border-b-brand focus-within:shadow-[inset_0_-1px_0_0_rgb(var(--brand))]",
          invalid && "border-danger", disabled && "bg-surface-2")}>
        {value.map((v) => (
          <span key={v} className="inline-flex items-center gap-1 rounded-[3px] border border-line bg-surface-2 py-0.5 pl-2 pr-1 text-xs text-fg">
            {renderChip ? renderChip(v) : (byValue.get(v)?.label ?? v)}
            {!disabled && (
              <button type="button" aria-label={`remove ${byValue.get(v)?.label ?? v}`} onClick={(e) => { e.stopPropagation(); onChange(value.filter((x) => x !== v)); }}
                className="grid h-4 w-4 place-items-center rounded text-fg-subtle hover:bg-surface-3 hover:text-fg"><X className="h-3 w-3" /></button>
            )}
          </span>
        ))}
        {!disabled && (
          <input ref={inputRef} role="combobox" aria-label={label} aria-expanded={open} aria-controls={listId} aria-autocomplete="list"
            aria-activedescendant={open && matches[hi] ? optId(hi) : undefined}
            value={q} placeholder={value.length ? "" : placeholder}
            onChange={(e) => { setQ(e.target.value); setOpen(true); setHi(0); }} onKeyDown={onKey}
            onFocus={() => setOpen(true)} onBlur={() => setTimeout(() => setOpen(false), 120)}
            className="min-w-[8rem] flex-1 bg-transparent px-1 py-0.5 text-xs text-fg placeholder:text-fg-subtle focus:outline-none" />
        )}
      </div>
      {open && !disabled && (
        <ul id={listId} role="listbox" aria-multiselectable="true" aria-label={label}
          className="absolute left-0 right-0 top-full z-40 mt-1 max-h-60 overflow-y-auto rounded-[3px] border border-line bg-surface py-1 shadow-pop">
          {matches.length === 0 && <li className="px-3 py-1.5 text-xs text-fg-subtle">–</li>}
          {matches.map((o, i) => {
            const sel = value.includes(o.value);
            return (
              <li key={o.value} id={optId(i)} role="option" aria-selected={sel}
                onMouseDown={(e) => { e.preventDefault(); toggle(o.value); }} onMouseEnter={() => setHi(i)}
                className={cn("flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm", i === hi && "bg-surface-3/70")}>
                <span className={cn("grid h-3.5 w-3.5 shrink-0 place-items-center rounded-[2px] border", sel ? "border-brand-solid bg-brand-solid text-on-brand" : "border-line-strong")}>
                  {sel && <Check className="h-3 w-3" aria-hidden />}
                </span>
                {o.prefix}
                <span className="min-w-0 flex-1 truncate">{o.label}</span>
                {o.hint && <span className="shrink-0 font-mono text-2xs text-fg-subtle">{o.hint}</span>}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/** Small set of toggle chips (aria-pressed) for short fixed option lists like HTTP methods. */
export function ChipToggles({ value, onChange, options, label, disabled, invalid }: {
  value: string[]; onChange: (v: string[]) => void; options: string[]; label: string; disabled?: boolean; invalid?: boolean;
}) {
  return (
    <div role="group" aria-label={label} className={cn("flex flex-wrap gap-1.5", invalid && "rounded-[3px] ring-1 ring-danger ring-offset-2 ring-offset-surface")}>
      {options.map((o) => {
        const on = value.includes(o);
        return (
          <button key={o} type="button" aria-pressed={on} disabled={disabled}
            onClick={() => onChange(on ? value.filter((x) => x !== o) : [...value, o])}
            className={cn("h-7 rounded-[3px] border px-2.5 font-mono text-xs transition-colors disabled:opacity-50",
              on ? "border-brand-solid bg-brand-solid text-on-brand" : "border-line-strong text-fg-muted hover:bg-surface-2 hover:text-fg")}>
            {o}
          </button>
        );
      })}
    </div>
  );
}
