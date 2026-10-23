"use client";

import "@xyflow/react/dist/style.css";

import { Background, Handle, MarkerType, Position, ReactFlow, ReactFlowProvider, useReactFlow, type Edge, type Node, type NodeProps } from "@xyflow/react";
import { Maximize, Minus, Plus } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";

import type { Locale } from "@/i18n/config";
import { cn } from "@/lib/cn";
import type { DependencyGraphData, DependencyGraphNode } from "@/lib/dependencies";
import { NODE_HEIGHT, NODE_WIDTH, placeNodes } from "@/lib/dependencyLayout";

import { Button } from "../ui/Button";
import { catalogHref, type CatalogLabels } from "../catalog/shared";

type Data = { item: DependencyGraphNode; focus: boolean; href: string | null; caption: string };

const shapes = {
  project: "rounded-lg border-kind-project bg-kind-project-soft text-kind-project",
  external: "rounded-full border-cobalt bg-cobalt-soft text-cobalt",
  unresolved: "rounded-md border-dashed border-amber bg-amber-soft text-amber",
} as const;

function GraphNode({ data }: NodeProps<Node<Data>>) {
  const { item, focus, href, caption } = data;
  const body = (
    <>
      <span className="block truncate font-medium text-ink">{item.name}</span>
      <span className="block truncate text-xs">{caption}</span>
    </>
  );
  const box = cn(
    "flex h-full flex-col justify-center border-2 px-3 text-left text-sm",
    shapes[item.type],
    !item.in_scope && !focus && "opacity-60",
    focus && "ring-2 ring-signal ring-offset-2 ring-offset-surface",
  );
  return (
    <div style={{ width: NODE_WIDTH, height: NODE_HEIGHT }}>
      <Handle type="target" position={Position.Left} className="!opacity-0" />
      {href ? (
        <Link href={href} className={cn(box, "focus-visible:outline-2 focus-visible:outline-signal")}>
          {body}
        </Link>
      ) : (
        <div className={box}>{body}</div>
      )}
      <Handle type="source" position={Position.Right} className="!opacity-0" />
    </div>
  );
}

const nodeTypes = { dependency: GraphNode };

function Zoom({ labels }: { labels: CatalogLabels }) {
  const t = labels.dependencies;
  const flow = useReactFlow();
  return (
    <div className="absolute top-2 right-2 z-10 flex gap-1">
      <Button size="sm" variant="secondary" onClick={() => void flow.fitView()} aria-label={t.fit}>
        <Maximize aria-hidden="true" />
        {t.fit}
      </Button>
      <Button size="icon" variant="secondary" onClick={() => void flow.zoomIn()} aria-label={t.zoomIn}>
        <Plus aria-hidden="true" />
      </Button>
      <Button size="icon" variant="secondary" onClick={() => void flow.zoomOut()} aria-label={t.zoomOut}>
        <Minus aria-hidden="true" />
      </Button>
    </div>
  );
}

type Props = { graph: DependencyGraphData; focus: string | null; locale: Locale; labels: CatalogLabels };

export default function DependencyGraphCanvas({ graph, focus, locale, labels }: Props) {
  const t = labels.dependencies;
  const { nodes, edges } = useMemo(() => {
    const placed = new Map(placeNodes(graph).map((p) => [p.id, p]));
    const nodes: Node<Data>[] = graph.nodes.map((item) => ({
      id: item.id,
      type: "dependency",
      position: { x: placed.get(item.id)?.x ?? 0, y: placed.get(item.id)?.y ?? 0 },
      data: {
        item,
        focus: item.id === focus,
        href: item.type === "project" ? catalogHref(locale, item.id, "dependencies") : null,
        caption: item.type === "project" ? item.path : item.type === "external" ? t.external : t.unresolved,
      },
      draggable: false,
      selectable: false,
    }));
    const edges: Edge[] = graph.edges.map((e) => ({
      id: `${e.from}->${e.to}`,
      source: e.from,
      target: e.to,
      markerEnd: { type: MarkerType.ArrowClosed },
      style: { stroke: "var(--muted)" },
    }));
    return { nodes, edges };
  }, [graph, focus, locale, t]);

  return (
    <div className="relative h-[28rem] w-full overflow-hidden rounded-card border border-line bg-surface" role="group" aria-label={t.graphLabel}>
      <ReactFlowProvider>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          nodeTypes={nodeTypes}
          fitView
          nodesConnectable={false}
          nodesDraggable={false}
          nodesFocusable={false}
          edgesFocusable={false}
          minZoom={0.2}
          style={{ background: "transparent" }}
        >
          <Background color="var(--line)" />
          <Zoom labels={labels} />
        </ReactFlow>
      </ReactFlowProvider>
    </div>
  );
}
