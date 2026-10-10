"use client";

import { useCallback, useEffect, useState, useSyncExternalStore } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";

import { apiGet, errorCode } from "./api";
import { nextRefreshMs, parseInterval, REFRESH_KEY, type RefreshInterval } from "./autoRefresh";

const listeners = new Set<() => void>();

function readInterval(): RefreshInterval {
  try {
    return parseInterval(window.localStorage.getItem(REFRESH_KEY));
  } catch {
    return 0;
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function useRefreshInterval(): [RefreshInterval, (v: RefreshInterval) => void] {
  const value = useSyncExternalStore(subscribe, readInterval, (): RefreshInterval => 0);
  const set = useCallback((v: RefreshInterval) => {
    try {
      window.localStorage.setItem(REFRESH_KEY, String(v));
    } catch {
      return;
    } finally {
      for (const l of listeners) l();
    }
  }, []);
  return [value, set];
}

type Loaded<T> = { key: string; data: T };
type Failed = { key: string; text: string };

export function usePolledGet<T>(path: `/${string}`, interval: RefreshInterval, errors: Messages["errors"]) {
  const [page, setPage] = useState<Loaded<T> | null>(null);
  const [failure, setFailure] = useState<Failed | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [tick, setTick] = useState(0);
  const [done, setDone] = useState(0);

  useEffect(() => {
    let alive = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setRefreshing(true);
    apiGet<T>(path)
      .then(
        (data) => {
          if (!alive) return;
          setPage({ key: path, data });
          setFailure(null);
        },
        (e: unknown) => alive && setFailure({ key: path, text: errorText(errors, errorCode(e)) }),
      )
      .finally(() => {
        if (!alive) return;
        setRefreshing(false);
        setDone((n) => n + 1);
      });
    return () => {
      alive = false;
    };
  }, [path, tick, errors]);

  useEffect(() => {
    const ms = nextRefreshMs(interval, document.visibilityState === "hidden");
    if (ms === null) return;
    const timer = setTimeout(() => {
      if (document.visibilityState !== "hidden") setTick((n) => n + 1);
    }, ms);
    return () => clearTimeout(timer);
  }, [interval, done]);

  useEffect(() => {
    if (interval === 0) return;
    const onVisible = () => {
      if (document.visibilityState === "visible") setTick((n) => n + 1);
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [interval]);

  const refresh = useCallback(() => setTick((n) => n + 1), []);
  return { page, failure, refreshing, refresh };
}
