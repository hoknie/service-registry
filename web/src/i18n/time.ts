import type { Locale } from "./config";

export function when(value: string | null | undefined, locale: Locale, never: string): string {
  return value ? new Date(value).toLocaleString(locale) : never;
}

const steps: [Intl.RelativeTimeFormatUnit, number][] = [
  ["second", 60],
  ["minute", 60],
  ["hour", 24],
  ["day", 7],
  ["week", 4.35],
  ["month", 12],
  ["year", Number.POSITIVE_INFINITY],
];

export function ago(value: string | null | undefined, locale: Locale, never: string, now = Date.now()): string {
  if (!value) return never;
  let amount = (new Date(value).getTime() - now) / 1000;
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });
  for (const [unit, size] of steps) {
    if (Math.abs(amount) < size) return rtf.format(Math.round(amount), unit);
    amount /= size;
  }
  return never;
}
