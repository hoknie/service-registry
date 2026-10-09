import { defaultLocale, isLocale, type Locale } from "./config";

export function pickLocale(acceptLanguage: string | null | undefined): Locale {
  if (!acceptLanguage) return defaultLocale;
  let best: { locale: Locale; q: number } | undefined;
  for (const entry of acceptLanguage.split(",")) {
    const [tag = "", ...params] = entry.split(";");
    const qParam = params.map((p) => p.trim()).find((p) => p.startsWith("q="));
    const q = qParam ? Number.parseFloat(qParam.slice(2)) : 1;
    const primary = tag.trim().split("-")[0]?.toLowerCase();
    if (!isLocale(primary) || !(q > 0)) continue;
    if (!best || q > best.q) best = { locale: primary, q };
  }
  return best?.locale ?? defaultLocale;
}
