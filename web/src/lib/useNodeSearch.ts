"use client";

import { useEffect, useState } from "react";

import { apiGet, type CatalogTable } from "./api";
import { nodeHits, nodeSearchQuery, type NodeHit } from "./nodeSearch";

const DELAY = 200;

export type NodeSearch = { hits: NodeHit[] | null; error: unknown };

export function useNodeSearch(query: string): NodeSearch {
  const [result, setResult] = useState<{ path: string | null; search: NodeSearch }>({ path: null, search: { hits: null, error: null } });
  const path = nodeSearchQuery(query);

  useEffect(() => {
    if (!path) return;
    const abort = new AbortController();
    const timer = setTimeout(() => {
      apiGet<CatalogTable>(path as `/${string}`, abort.signal)
        .then((table) => setResult({ path, search: { hits: nodeHits(table), error: null } }))
        .catch((error: unknown) => {
          if (!abort.signal.aborted) setResult({ path, search: { hits: null, error } });
        });
    }, DELAY);
    return () => {
      clearTimeout(timer);
      abort.abort();
    };
  }, [path]);

  if (!path) return { hits: null, error: null };
  return result.path === path ? result.search : { hits: null, error: null };
}
