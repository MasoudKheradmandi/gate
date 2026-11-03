import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { ko } from "./locales/ko";
import { en } from "./locales/en";
import { zh } from "./locales/zh";
import { ja } from "./locales/ja";
import { ru } from "./locales/ru";
import { fr } from "./locales/fr";
import { de } from "./locales/de";
import { es } from "./locales/es";

/** Supported console languages. English is the default and the fallback for missing keys. */
export const LANGS = ["en", "ko", "zh", "ja", "ru", "fr", "de", "es"] as const;
export type Lang = (typeof LANGS)[number];
export type Key = keyof typeof ko;

/** Native language names for pickers. */
export const LANG_NAMES: Record<Lang, string> = {
  en: "English", ko: "한국어", zh: "简体中文", ja: "日本語", ru: "Русский", fr: "Français", de: "Deutsch", es: "Español",
};

/** BCP 47 tags for Intl (dates, numbers, region names). */
export const LOCALE_TAG: Record<Lang, string> = {
  en: "en-US", ko: "ko-KR", zh: "zh-CN", ja: "ja-JP", ru: "ru-RU", fr: "fr-FR", de: "de-DE", es: "es-ES",
};
export const localeTag = (l: Lang) => LOCALE_TAG[l] ?? "en-US";

const dicts: Record<Lang, Partial<Record<Key, string>>> = { en, ko, zh, ja, ru, fr, de, es };

export const isLang = (v: unknown): v is Lang => typeof v === "string" && (LANGS as readonly string[]).includes(v);

function initialLang(): Lang {
  try {
    const saved = localStorage.getItem("corvus.lang");
    if (isLang(saved)) return saved;
  } catch { /* storage unavailable */ }
  return "en";
}

export type TFn = (k: Key, vars?: Record<string, string | number>) => string;
const Ctx = createContext<{ lang: Lang; setLang: (l: Lang) => void; t: TFn }>(null!);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(initialLang);
  useEffect(() => { document.documentElement.lang = lang; }, [lang]);
  const setLang = useCallback((l: Lang) => {
    setLangState(l);
    try { localStorage.setItem("corvus.lang", l); } catch { /* ignore */ }
  }, []);
  const t = useCallback<TFn>((k, vars) => {
    const s = dicts[lang][k] ?? en[k] ?? k;
    return vars ? s.replace(/\{(\w+)\}/g, (_, n) => String(vars[n] ?? "")) : s;
  }, [lang]);
  const value = useMemo(() => ({ lang, setLang, t }), [lang, setLang, t]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export const useI18n = () => useContext(Ctx);
