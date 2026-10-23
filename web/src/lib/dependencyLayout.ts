import { graphlib, layout } from "@dagrejs/dagre";

import type { DependencyGraphData } from "./dependencies";

export const NODE_WIDTH = 200;
export const NODE_HEIGHT = 56;

export type Placed = { id: string; x: number; y: number };

export function placeNodes(graph: DependencyGraphData): Placed[] {
  const g = new graphlib.Graph();
  g.setGraph({ rankdir: "LR", nodesep: 24, ranksep: 72 });
  g.setDefaultEdgeLabel(() => ({}));
  for (const n of graph.nodes) g.setNode(n.id, { width: NODE_WIDTH, height: NODE_HEIGHT });
  for (const e of graph.edges) g.setEdge(e.from, e.to);
  layout(g);
  return graph.nodes.map((n) => {
    const p = g.node(n.id);
    return { id: n.id, x: p.x - NODE_WIDTH / 2, y: p.y - NODE_HEIGHT / 2 };
  });
}
