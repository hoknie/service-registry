export type PollingMode = "on" | "off" | "published";

export const POLLING_MODES: PollingMode[] = ["on", "off", "published"];

export type PollingOrigin = { id: string; kind: string; name: string } | null;

export type PollingField = "interval_secs" | "timeout_secs" | "mode";

export type Polling = {
  interval_secs: number | null;
  timeout_secs: number;
  mode: PollingMode;
  own: Partial<{ interval_secs: number; timeout_secs: number; mode: PollingMode }>;
  from: Record<PollingField, PollingOrigin>;
  defaults: { repo_sync_interval_secs: number; docs_interval_secs: number; skills_interval_secs: number; timeout_secs: number };
};

export type PollingDraft = {
  interval: { set: boolean; minutes: string };
  timeout: { set: boolean; seconds: string };
  mode: { set: boolean; value: PollingMode };
};

export const MIN_INTERVAL_MINUTES = 1;
export const MAX_INTERVAL_MINUTES = 1440;
export const MIN_TIMEOUT_SECS = 1;
export const MAX_TIMEOUT_SECS = 600;

export function minutesOf(secs: number): string {
  return String(Math.round(secs / 60));
}

export function draftOf(p: Polling, fallbackInterval: number): PollingDraft {
  return {
    interval: { set: "interval_secs" in p.own, minutes: minutesOf(p.own.interval_secs ?? p.interval_secs ?? fallbackInterval) },
    timeout: { set: "timeout_secs" in p.own, seconds: String(p.own.timeout_secs ?? p.timeout_secs) },
    mode: { set: "mode" in p.own, value: p.own.mode ?? p.mode },
  };
}

function wholeIn(text: string, min: number, max: number): number | null {
  if (!/^\d+$/.test(text.trim())) return null;
  const n = Number(text.trim());
  return n >= min && n <= max ? n : null;
}

export function draftErrors(d: PollingDraft): { interval: boolean; timeout: boolean } {
  return {
    interval: d.interval.set && wholeIn(d.interval.minutes, MIN_INTERVAL_MINUTES, MAX_INTERVAL_MINUTES) === null,
    timeout: d.timeout.set && wholeIn(d.timeout.seconds, MIN_TIMEOUT_SECS, MAX_TIMEOUT_SECS) === null,
  };
}

export function pollingBody(d: PollingDraft): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  const minutes = wholeIn(d.interval.minutes, MIN_INTERVAL_MINUTES, MAX_INTERVAL_MINUTES);
  if (d.interval.set && minutes !== null) body.interval_secs = minutes * 60;
  const seconds = wholeIn(d.timeout.seconds, MIN_TIMEOUT_SECS, MAX_TIMEOUT_SECS);
  if (d.timeout.set && seconds !== null) body.timeout_secs = seconds;
  if (d.mode.set) body.mode = d.mode.value;
  return body;
}
