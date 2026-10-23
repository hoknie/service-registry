import { apiGet, apiSend } from "./api";
import type { Dependencies, DependencyGraphData, DependencyItem } from "./dependencies";

export function getDependencies(nodeId: string, signal?: AbortSignal): Promise<Dependencies> {
  return apiGet<Dependencies>(`/v1/catalog/nodes/${nodeId}/dependencies`, signal);
}

export function getDependencyGraph(nodeId: string, depth: number, signal?: AbortSignal): Promise<DependencyGraphData> {
  return apiGet<DependencyGraphData>(`/v1/catalog/nodes/${nodeId}/dependencies/graph?depth=${depth}`, signal);
}

export function addDependency(nodeId: string, target: string, note: string): Promise<DependencyItem> {
  return apiSend<DependencyItem>("POST", `/v1/catalog/nodes/${nodeId}/dependencies`, { target, note: note.trim() || null });
}

export function removeDependency(nodeId: string, id: string): Promise<void> {
  return apiSend("DELETE", `/v1/catalog/nodes/${nodeId}/dependencies/${id}`);
}
