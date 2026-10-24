import { apiGet, apiSend, type Group, type GroupDetails, type Items, type MemberCandidate, type OrganizationGroups } from "./api";

const base = (nodeId: string) => `/v1/catalog/nodes/${nodeId}/groups` as const;

export function organizationGroups(nodeId: string, signal?: AbortSignal): Promise<OrganizationGroups> {
  return apiGet<OrganizationGroups>(`${base(nodeId)}?limit=200`, signal);
}

export function organizationGroup(nodeId: string, groupId: string, signal?: AbortSignal): Promise<GroupDetails> {
  return apiGet<GroupDetails>(`${base(nodeId)}/${groupId}`, signal);
}

export function createOrganizationGroup(nodeId: string, name: string): Promise<Group> {
  return apiSend<Group>("POST", base(nodeId), { name });
}

export function renameOrganizationGroup(nodeId: string, groupId: string, name: string): Promise<Group> {
  return apiSend<Group>("PATCH", `${base(nodeId)}/${groupId}`, { name });
}

export function deleteOrganizationGroup(nodeId: string, groupId: string): Promise<void> {
  return apiSend<void>("DELETE", `${base(nodeId)}/${groupId}`);
}

export function setOrganizationMember(nodeId: string, groupId: string, userId: string, member: boolean): Promise<void> {
  return apiSend<void>(member ? "PUT" : "DELETE", `${base(nodeId)}/${groupId}/members/${userId}`);
}

export async function organizationCandidates(nodeId: string, groupId: string, q: string, signal?: AbortSignal): Promise<MemberCandidate[]> {
  const path = `${base(nodeId)}/${groupId}/candidates?q=${encodeURIComponent(q.trim())}` as `/${string}`;
  return (await apiGet<Items<MemberCandidate>>(path, signal)).items;
}
