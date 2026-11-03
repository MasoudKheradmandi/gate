import { SYMBOL_PATH, SYMBOL_VIEWBOX, WORDMARK_PATH, WORDMARK_VIEWBOX } from "./brand";

/** Corvus symbol (raven head). Renders in currentColor. */
export function Logo({ className }: { className?: string }) {
  return (
    <svg viewBox={SYMBOL_VIEWBOX} className={className} role="img" aria-label="Corvus" fill="currentColor">
      <path fillRule="evenodd" d={SYMBOL_PATH} />
    </svg>
  );
}

/** Corvus symbol + "CORVUS" wordmark. Renders in currentColor; size it by height. */
export function Wordmark({ className }: { className?: string }) {
  return (
    <svg viewBox={WORDMARK_VIEWBOX} className={className} role="img" aria-label="Corvus" fill="currentColor">
      <path fillRule="evenodd" d={WORDMARK_PATH} />
    </svg>
  );
}
