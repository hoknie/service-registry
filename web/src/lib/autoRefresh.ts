export const REFRESH_KEY = "svcr-scans-refresh";
export const REFRESH_INTERVALS = [0, 10, 30, 60] as const;
export type RefreshInterval = (typeof REFRESH_INTERVALS)[number];

export function parseInterval(raw: string | null | undefined): RefreshInterval {
  const n = Number(raw);
  return (REFRESH_INTERVALS as readonly number[]).includes(n) ? (n as RefreshInterval) : 0;
}

export function nextRefreshMs(interval: RefreshInterval, hidden: boolean): number | null {
  if (interval === 0 || hidden) return null;
  return interval * 1000;
}
