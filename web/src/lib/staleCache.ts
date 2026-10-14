export type Entry<T> = { value?: T; error?: string; loading: boolean; stale?: boolean };

export type Cache<T> = {
  get: (key: string) => Entry<T> | undefined;
  read: (key: string) => Entry<T> | undefined;
  ensure: (key: string, load: () => Promise<T>, retry?: boolean) => void;
  revalidate: () => void;
  values: () => IterableIterator<Entry<T>>;
};

export function staleCache<T>(emit: () => void, codeOf: (e: unknown) => string): Cache<T> {
  const entries = new Map<string, Entry<T>>();
  const loaders = new Map<string, () => Promise<T>>();

  const refresh = (key: string) => {
    const cur = entries.get(key);
    const load = loaders.get(key);
    if (!cur || !load || cur.loading) return;
    entries.set(key, { ...cur, stale: false });
    load()
      .then((value) => entries.set(key, { value, loading: false }))
      .catch((e: unknown) => entries.set(key, cur.value !== undefined ? { value: cur.value, loading: false } : { error: codeOf(e), loading: false }))
      .finally(emit);
  };

  return {
    get: (key) => entries.get(key),
    read: (key) => {
      const e = entries.get(key);
      if (e?.stale) queueMicrotask(() => refresh(key));
      return e;
    },
    ensure: (key, load, retry = false) => {
      loaders.set(key, load);
      const cur = entries.get(key);
      if (cur?.stale && !retry) return refresh(key);
      if (cur?.loading || (!retry && cur && (cur.value !== undefined || cur.error !== undefined))) return;
      entries.set(key, { loading: true });
      queueMicrotask(emit);
      load()
        .then((value) => entries.set(key, { value, loading: false }))
        .catch((e: unknown) => entries.set(key, { error: codeOf(e), loading: false }))
        .finally(emit);
    },
    values: () => entries.values(),
    revalidate: () => {
      for (const [key, e] of entries) if (!e.loading) entries.set(key, { ...e, stale: true });
    },
  };
}
