import { apiGet, apiSend, type Contact, type Group, type Items, type OwnGroup, type OwnerCandidate, type OwnerSet, type OwnerSuggestion } from "./api";
import { ownersBody, type DraftOwner } from "./owners";

export function getOwners(nodeId: string, signal?: AbortSignal): Promise<OwnerSet> {
  return apiGet<OwnerSet>(`/v1/catalog/nodes/${nodeId}/owners`, signal);
}

export function putOwners(nodeId: string, draft: DraftOwner[]): Promise<OwnerSet> {
  return apiSend<OwnerSet>("PUT", `/v1/catalog/nodes/${nodeId}/owners`, ownersBody(draft));
}

export function clearOwners(nodeId: string): Promise<OwnerSet> {
  return apiSend<OwnerSet>("PUT", `/v1/catalog/nodes/${nodeId}/owners`, { owners: [], none: true });
}

export async function ownerCandidates(nodeId: string, q: string, signal?: AbortSignal): Promise<OwnerCandidate[]> {
  if (q.trim().length < 2) return [];
  const path = `/v1/catalog/nodes/${nodeId}/owners/candidates?q=${encodeURIComponent(q.trim())}` as `/${string}`;
  return (await apiGet<Items<OwnerCandidate>>(path, signal)).items;
}

export async function suggestOwners(q: string, signal?: AbortSignal): Promise<OwnerSuggestion[]> {
  const path = `/v1/catalog/owners?q=${encodeURIComponent(q.trim())}` as `/${string}`;
  return (await apiGet<Items<OwnerSuggestion>>(path, signal)).items;
}

export function putGroupContacts(groupId: string, contacts: Contact[], onCallUrl: string | null): Promise<Group> {
  return apiSend<Group>("PUT", `/v1/groups/${groupId}/contacts`, { contacts, on_call_url: onCallUrl });
}

export type SaveContacts = (groupId: string, contacts: Contact[], onCallUrl: string | null) => Promise<Group>;

export async function ownGroups(signal?: AbortSignal): Promise<OwnGroup[]> {
  return (await apiGet<Items<OwnGroup>>("/v1/account/groups", signal)).items;
}

export function putOwnGroupContacts(groupId: string, contacts: Contact[], onCallUrl: string | null): Promise<OwnGroup> {
  return apiSend<OwnGroup>("PUT", `/v1/account/groups/${groupId}/contacts`, { contacts, on_call_url: onCallUrl });
}

export function setContactEditor(groupId: string, userId: string, on: boolean): Promise<unknown> {
  return apiSend(on ? "PUT" : "DELETE", `/v1/groups/${groupId}/contact-editors/${userId}`);
}
