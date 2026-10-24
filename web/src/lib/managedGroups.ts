import { apiGet, apiSend, type Items, type ManagedGroup, type ManagedGroupDetails, type MemberCandidate } from "./api";

export async function managedGroups(signal?: AbortSignal): Promise<ManagedGroup[]> {
  return (await apiGet<Items<ManagedGroup>>("/v1/account/managed-groups", signal)).items;
}

export function managedGroup(id: string, signal?: AbortSignal): Promise<ManagedGroupDetails> {
  return apiGet<ManagedGroupDetails>(`/v1/account/managed-groups/${id}`, signal);
}

export async function memberCandidates(groupId: string, q: string, signal?: AbortSignal): Promise<MemberCandidate[]> {
  const path = `/v1/account/managed-groups/${groupId}/candidates?q=${encodeURIComponent(q.trim())}` as `/${string}`;
  return (await apiGet<Items<MemberCandidate>>(path, signal)).items;
}

export function setMember(groupId: string, userId: string, member: boolean): Promise<void> {
  return apiSend<void>(member ? "PUT" : "DELETE", `/v1/account/managed-groups/${groupId}/members/${userId}`);
}
