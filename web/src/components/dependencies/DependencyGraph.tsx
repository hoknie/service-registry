"use client";

import { Network } from "lucide-react";
import { lazy, Suspense, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { CatalogNode } from "@/lib/api";
import type { DependencyGraphData } from "@/lib/dependencies";
import { getDependencyGraph } from "@/lib/dependenciesApi";

import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { Skeleton } from "../ui/Skeleton";
import type { CatalogLabels } from "../catalog/shared";

const Canvas = lazy(() => import("./DependencyGraphCanvas"));

type Props = { node: CatalogNode; depth: number; locale: Locale; labels: CatalogLabels; fail: (e: unknown) => string };

export function DependencyGraph({ node, depth, locale, labels, fail }: Props) {
  const t = labels.dependencies;
  const [state, setState] = useState<{ key: string; graph: DependencyGraphData | null; error: string | null } | null>(null);
  const key = `${node.id}:${depth}`;

  useEffect(() => {
    const abort = new AbortController();
    getDependencyGraph(node.id, depth, abort.signal)
      .then((graph) => setState({ key, graph, error: null }))
      .catch((e: unknown) => {
        if (!abort.signal.aborted) setState({ key, graph: null, error: fail(e) });
      });
    return () => abort.abort();
  }, [node.id, depth, key, fail]);

  const fallback = <Skeleton className="h-[28rem] w-full" />;
  if (!state || state.key !== key) return fallback;
  if (state.error) return <Message note={{ kind: "error", text: state.error }} />;
  const graph = state.graph;
  if (!graph || graph.edges.length === 0) {
    return <EmptyState icon={Network} title={node.kind === "project" ? t.emptyDependsOn : t.emptyFolder} text={t.howTo} />;
  }
  return (
    <div className="grid gap-2">
      {(graph.truncated || graph.hidden_edges > 0) && (
        <p className="flex flex-wrap gap-3 text-sm text-muted">
          {graph.truncated && <span>{t.truncated}</span>}
          {graph.hidden_edges > 0 && <span>{format(t.hiddenEdges, { count: graph.hidden_edges })}</span>}
        </p>
      )}
      <Suspense fallback={fallback}>
        <Canvas graph={graph} focus={node.kind === "project" ? node.id : null} locale={locale} labels={labels} />
      </Suspense>
    </div>
  );
}
