import type { Lang } from "../i18n";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { cn, fmtClock, toMs } from "../lib/utils";

/** Measures an element's width (ResizeObserver). */
function useWidth<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [w, setW] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    setW(el.clientWidth);
    const ro = new ResizeObserver(() => setW(el.clientWidth));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, w] as const;
}

const smooth = (pts: [number, number][]): string => {
  if (pts.length < 2) return pts.length ? `M${pts[0][0]},${pts[0][1]}` : "";
  let d = `M${pts[0][0]},${pts[0][1]}`;
  for (let i = 0; i < pts.length - 1; i++) {
    const [x0, y0] = pts[i], [x1, y1] = pts[i + 1];
    const cx = (x0 + x1) / 2;
    d += ` C${cx},${y0} ${cx},${y1} ${x1},${y1}`;
  }
  return d;
};

/** Tiny trend line with soft area fill. Colour comes from `color` (CSS colour, e.g. rgb(var(--chart-1))). */
export function Sparkline({ values, color = "rgb(var(--chart-1))", height = 32, className }: {
  values: number[]; color?: string; height?: number; className?: string;
}) {
  const id = useId();
  const [ref, w] = useWidth<HTMLDivElement>();
  const path = useMemo(() => {
    if (values.length < 2 || w === 0) return null;
    const max = Math.max(...values, 1e-9), min = Math.min(...values, 0);
    const span = max - min || 1;
    const pts = values.map((v, i): [number, number] => [(i / (values.length - 1)) * w, height - 2 - ((v - min) / span) * (height - 6)]);
    const line = smooth(pts);
    return { line, area: `${line} L${w},${height} L0,${height} Z` };
  }, [values, w, height]);
  return (
    <div ref={ref} className={cn("w-full min-w-0", className)} style={{ height }} aria-hidden>
      {path && (
        <svg width="100%" height={height} viewBox={`0 0 ${w} ${height}`} preserveAspectRatio="none" className="block overflow-visible">
          <defs>
            <linearGradient id={id} x1="0" x2="0" y1="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity="0.28" />
              <stop offset="100%" stopColor={color} stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={path.area} fill={`url(#${id})`} />
          <path d={path.line} fill="none" stroke={color} strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" vectorEffect="non-scaling-stroke" />
        </svg>
      )}
    </div>
  );
}

export interface Series { key: string; label: string; color: string }
export interface ChartPoint { t: number | string; v: Record<string, number> }

/** Time chart: stacked areas or overlaid lines, hover crosshair + tooltip, responsive. */
export function TimeChart({ points, series, mode = "stacked", height = 220, format = (n) => String(Math.round(n * 10) / 10), empty }: {
  points: ChartPoint[]; series: Series[]; mode?: "stacked" | "lines"; height?: number;
  format?: (n: number) => string; lang?: Lang; empty?: string;
}) {
  const uid = useId();
  const [ref, w] = useWidth<HTMLDivElement>();
  const [hover, setHover] = useState<number | null>(null);
  const padL = 40, padR = 8, padT = 8, padB = 22;

  const geo = useMemo(() => {
    const n = points.length;
    if (n < 2 || w === 0) return null;
    const iw = Math.max(w - padL - padR, 10), ih = height - padT - padB;
    const times = points.map((p) => toMs(p.t));
    const totals = points.map((p) => mode === "stacked" ? series.reduce((a, s) => a + (p.v[s.key] ?? 0), 0) : Math.max(...series.map((s) => p.v[s.key] ?? 0)));
    const rawMax = Math.max(...totals, 1);
    const pow = Math.pow(10, Math.floor(Math.log10(rawMax)));
    const step = [1, 2, 2.5, 5, 10].map((m) => m * pow).find((s) => rawMax / s <= 4) ?? 10 * pow;
    const max = Math.ceil(rawMax / step) * step;
    const x = (i: number) => padL + (i / (n - 1)) * iw;
    const y = (v: number) => padT + ih - (v / max) * ih;
    const ticks = Array.from({ length: Math.round(max / step) + 1 }, (_, i) => i * step);
    let layers: { s: Series; line: string; area: string }[];
    if (mode === "stacked") {
      const base = new Array(n).fill(0);
      layers = series.map((s) => {
        const lo = [...base];
        points.forEach((p, i) => { base[i] += p.v[s.key] ?? 0; });
        const top = points.map((_, i): [number, number] => [x(i), y(base[i])]);
        const bot = lo.map((v, i): [number, number] => [x(i), y(v)]).reverse();
        const line = smooth(top);
        const area = `${line} L${bot[0][0]},${bot[0][1]} ${smooth(bot).replace(/^M[^C]*/, "")} Z`;
        return { s, line, area };
      });
    } else {
      layers = series.map((s) => {
        const line = smooth(points.map((p, i): [number, number] => [x(i), y(p.v[s.key] ?? 0)]));
        return { s, line, area: "" };
      });
    }
    return { x, y, ticks, layers, times, iw, ih };
  }, [points, series, mode, w, height]);

  if (points.length < 2) {
    return <div className="grid place-items-center text-sm text-fg-subtle" style={{ height }}>{empty}</div>;
  }
  const fmtT = (ms: number, long = false) => fmtClock(ms, long);

  const onMove = (e: React.PointerEvent<SVGSVGElement>) => {
    if (!geo) return;
    const r = e.currentTarget.getBoundingClientRect();
    const x = (e.clientX - r.left) * (w / (r.width || w)); // client px → viewBox units
    const rel = (x - padL) / geo.iw;
    setHover(Math.min(points.length - 1, Math.max(0, Math.round(rel * (points.length - 1)))));
  };

  const hp = hover != null ? points[hover] : null;
  const tipLeft = geo && hover != null ? geo.x(hover) : 0;
  return (
    <div ref={ref} className="relative w-full min-w-0 select-none" style={{ height }}>
      {geo && (
        <svg width="100%" height={height} viewBox={`0 0 ${w} ${height}`} className="block touch-none" onPointerMove={onMove} onPointerLeave={() => setHover(null)} role="img">
          <defs>
            {geo.layers.map((l, i) => (
              <linearGradient key={i} id={`${uid}-${i}`} x1="0" x2="0" y1="0" y2="1">
                <stop offset="0%" stopColor={l.s.color} stopOpacity={mode === "stacked" ? 0.5 : 0.2} />
                <stop offset="100%" stopColor={l.s.color} stopOpacity={mode === "stacked" ? 0.12 : 0} />
              </linearGradient>
            ))}
          </defs>
          {geo.ticks.map((tk) => (
            <g key={tk}>
              <line x1={padL} x2={w - padR} y1={geo.y(tk)} y2={geo.y(tk)} stroke="rgb(var(--line))" strokeDasharray={tk === 0 ? undefined : "2 4"} />
              <text x={padL - 8} y={geo.y(tk) + 3.5} textAnchor="end" className="fill-fg-subtle text-[10px]">{format(tk)}</text>
            </g>
          ))}
          {[0, 0.25, 0.5, 0.75, 1].map((f) => {
            const i = Math.round(f * (points.length - 1));
            return <text key={f} x={geo.x(i)} y={height - 6} textAnchor={f === 0 ? "start" : f === 1 ? "end" : "middle"} className="fill-fg-subtle text-[10px]">{fmtT(geo.times[i])}</text>;
          })}
          {geo.layers.map((l, i) => (
            <g key={l.s.key}>
              {mode === "stacked" && <path d={l.area} fill={`url(#${uid}-${i})`} />}
              <path d={l.line} fill="none" stroke={l.s.color} strokeWidth={mode === "stacked" ? 1.25 : 1.75} strokeLinejoin="round" strokeLinecap="round" />
            </g>
          ))}
          {hover != null && (
            <g pointerEvents="none">
              <line x1={geo.x(hover)} x2={geo.x(hover)} y1={padT} y2={height - padB} stroke="rgb(var(--fg-subtle))" strokeDasharray="3 3" />
              {mode === "lines" && series.map((s) => <circle key={s.key} cx={geo.x(hover)} cy={geo.y(points[hover].v[s.key] ?? 0)} r="3.5" fill={s.color} stroke="rgb(var(--surface))" strokeWidth="2" />)}
            </g>
          )}
        </svg>
      )}
      {hp && geo && (
        <div className="pointer-events-none absolute top-1 z-10 min-w-[9rem] rounded-[3px] border border-line bg-surface px-3 py-2 text-xs shadow-pop"
          style={{ left: Math.min(Math.max(tipLeft + 12, 4), Math.max(w - 170, 4)) }}>
          <div className="mb-1 font-medium text-fg num">{fmtT(geo.times[hover!], true)}</div>
          {series.map((s) => (
            <div key={s.key} className="flex items-center justify-between gap-4 py-px">
              <span className="flex items-center gap-1.5 text-fg-muted"><span className="h-2 w-2 rounded-sm" style={{ background: s.color }} />{s.label}</span>
              <span className="num font-medium text-fg">{format(hp.v[s.key] ?? 0)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function ChartLegend({ series }: { series: Series[] }) {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
      {series.map((s) => (
        <span key={s.key} className="flex items-center gap-1.5 text-xs text-fg-muted">
          <span aria-hidden className="h-2 w-2 rounded-sm" style={{ background: s.color }} />{s.label}
        </span>
      ))}
    </div>
  );
}

/** Circular occupancy gauge. */
export function RadialGauge({ value, max, size = 72, stroke = 7, label, sublabel }: {
  value: number; max: number; size?: number; stroke?: number; label?: string; sublabel?: string;
}) {
  const r = (size - stroke) / 2, c = 2 * Math.PI * r;
  const ratio = max > 0 ? Math.min(value / max, 1) : 0;
  const color = ratio >= 0.9 ? "rgb(var(--danger))" : ratio >= 0.7 ? "rgb(var(--warn))" : "rgb(var(--chart-2))";
  return (
    <div className="relative shrink-0" style={{ width: size, height: size }} role="img" aria-label={`${Math.round(ratio * 100)}%`}>
      <svg width={size} height={size} className="-rotate-90">
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="rgb(var(--surface-3))" strokeWidth={stroke} />
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={stroke} strokeLinecap="round"
          strokeDasharray={c} strokeDashoffset={c * (1 - ratio)} style={{ transition: "stroke-dashoffset .6s cubic-bezier(.22,1,.36,1), stroke .3s" }} />
      </svg>
      <div className="absolute inset-0 grid place-content-center text-center leading-none">
        <div className="num text-sm font-semibold text-fg">{label ?? `${Math.round(ratio * 100)}%`}</div>
        {sublabel && <div className="mt-0.5 text-[9px] text-fg-subtle">{sublabel}</div>}
      </div>
    </div>
  );
}

/** Horizontal share bar, e.g. 2xx/3xx/4xx/5xx composition. */
export function StackBar({ parts }: { parts: { value: number; color: string; label: string }[] }) {
  const total = parts.reduce((a, p) => a + p.value, 0);
  return (
    <div className="flex h-1.5 w-full overflow-hidden rounded-full bg-surface-3" role="img" aria-label={parts.map((p) => `${p.label} ${p.value}`).join(", ")}>
      {total > 0 && parts.map((p) => <div key={p.label} style={{ width: `${(p.value / total) * 100}%`, background: p.color }} />)}
    </div>
  );
}
