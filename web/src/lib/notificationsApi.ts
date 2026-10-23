import { apiGet, apiSend, type DirectoryEnvironment, type Items } from "./api";
import type { Suggestion } from "./labelSuggest";
import type { Subscription, SubscriptionBody, TestResult } from "./notifications";

const base = (nodeId: string) => `/v1/catalog/nodes/${nodeId}/notification-subscriptions` as const;

export async function listSubscriptions(nodeId: string): Promise<Subscription[]> {
  return (await apiGet<Items<Subscription>>(base(nodeId))).items;
}

export function putSubscription(nodeId: string, key: string, body: SubscriptionBody): Promise<Subscription> {
  return apiSend<Subscription>("PUT", `${base(nodeId)}/${encodeURIComponent(key)}` as `/${string}`, body);
}

export function deleteSubscription(nodeId: string, key: string): Promise<void> {
  return apiSend("DELETE", `${base(nodeId)}/${encodeURIComponent(key)}` as `/${string}`);
}

export function testSubscription(nodeId: string, key: string): Promise<TestResult> {
  return apiSend<TestResult>("POST", `${base(nodeId)}/${encodeURIComponent(key)}/test` as `/${string}`);
}

export function createSigningSecret(nodeId: string, key: string): Promise<{ secret: string; fingerprint: string }> {
  return apiSend("POST", `${base(nodeId)}/${encodeURIComponent(key)}/signing-secret` as `/${string}`);
}

let environments: Promise<string[]> | null = null;

export async function suggestEnvironments(draft: string, signal: AbortSignal): Promise<Suggestion[]> {
  environments ??= apiGet<Items<DirectoryEnvironment>>("/v1/environments", signal)
    .then((r) => r.items.map((e) => e.key))
    .catch(() => {
      environments = null;
      return [];
    });
  const q = draft.trim().toLowerCase();
  return (await environments).filter((k) => k.startsWith(q)).map((k) => ({ value: k, label: k }));
}
