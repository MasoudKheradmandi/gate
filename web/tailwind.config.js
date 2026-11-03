const c = (n) => `rgb(var(--${n}) / <alpha-value>)`;

/** @type {import('tailwindcss').Config} */
export default {
  darkMode: "class",
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg: c("bg"), surface: c("surface"), "surface-2": c("surface-2"), "surface-3": c("surface-3"),
        line: c("line"), "line-strong": c("line-strong"),
        fg: c("fg"), "fg-muted": c("fg-muted"), "fg-subtle": c("fg-subtle"),
        brand: c("brand"), "brand-solid": c("brand-solid"), "on-brand": c("on-brand"),
        ok: c("ok"), warn: c("warn"), danger: c("danger"), info: c("info"),
        chart: { 1: c("chart-1"), 2: c("chart-2"), 3: c("chart-3"), 4: c("chart-4"), 5: c("chart-5") },
      },
      fontFamily: {
        sans: ['"Pretendard Variable"', "Pretendard", "-apple-system", "BlinkMacSystemFont", "system-ui", "Segoe UI", "Roboto", '"Noto Sans KR"', '"Malgun Gothic"', "sans-serif"],
        mono: ["ui-monospace", '"SF Mono"', "Menlo", "Consolas", '"D2Coding"', '"Liberation Mono"', "monospace"],
      },
      fontSize: { "2xs": ["11px", "16px"], xs: ["12px", "16px"], sm: ["13px", "20px"], base: ["14px", "22px"] },
      borderRadius: { sm: "2px", DEFAULT: "3px", md: "3px", lg: "3px", xl: "4px", "2xl": "4px" },
      boxShadow: {
        card: "none",
        pop: "0 4px 12px 0 rgb(0 0 0 / 0.22)",
        sheet: "-4px 0 16px 0 rgb(0 0 0 / 0.25)",
      },
      keyframes: {
        "fade-in": { from: { opacity: "0" }, to: { opacity: "1" } },
        "fade-out": { from: { opacity: "1" }, to: { opacity: "0" } },
        "slide-in-right": { from: { transform: "translateX(100%)" }, to: { transform: "translateX(0)" } },
        "slide-out-right": { from: { transform: "translateX(0)" }, to: { transform: "translateX(100%)" } },
        "slide-in-left": { from: { transform: "translateX(-100%)" }, to: { transform: "translateX(0)" } },
        "slide-out-left": { from: { transform: "translateX(0)" }, to: { transform: "translateX(-100%)" } },
        "pop-in": { from: { opacity: "0", transform: "translateY(4px) scale(.97)" }, to: { opacity: "1", transform: "none" } },
        "rise": { from: { opacity: "0", transform: "translateY(6px)" }, to: { opacity: "1", transform: "none" } },
        shimmer: { "100%": { transform: "translateX(100%)" } },
        "pulse-ring": { "0%": { boxShadow: "0 0 0 0 currentColor" }, "70%": { boxShadow: "0 0 0 5px transparent" }, "100%": { boxShadow: "0 0 0 0 transparent" } },
      },
      animation: {
        "fade-in": "fade-in .15s ease-out", "fade-out": "fade-out .12s ease-in",
        "slide-in-right": "slide-in-right .2s ease-out", "slide-out-right": "slide-out-right .16s ease-in",
        "slide-in-left": "slide-in-left .22s cubic-bezier(.22,1,.36,1)", "slide-out-left": "slide-out-left .16s ease-in",
        "pop-in": "pop-in .14s ease-out", rise: "none",
        shimmer: "shimmer 1.6s infinite", "pulse-ring": "pulse-ring 2s infinite",
      },
    },
  },
  plugins: [],
};
