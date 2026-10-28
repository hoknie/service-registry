export const EVENT_TYPES = [
  "deployment.created",
  "deployment.drift",
  "deployment.drift_resolved",
  "link.check_failed",
  "link.check_recovered",
  "repo_sync.failed",
  "repo_sync.recovered",
] as const;
export type EventType = (typeof EVENT_TYPES)[number];

export const RECOVERY_OF: Partial<Record<EventType, EventType>> = {
  "deployment.drift": "deployment.drift_resolved",
  "link.check_failed": "link.check_recovered",
  "repo_sync.failed": "repo_sync.recovered",
};

const RECOVERIES = new Set(Object.values(RECOVERY_OF));

export const PRIMARY_EVENTS = EVENT_TYPES.filter((e) => !RECOVERIES.has(e));

export function toggleEvent(events: EventType[], e: EventType): EventType[] {
  if (!events.includes(e)) return EVENT_TYPES.filter((x) => x === e || events.includes(x));
  const recovery = RECOVERY_OF[e];
  return events.filter((x) => x !== e && x !== recovery);
}
export type JournalEventType = EventType | "test";

export const CHANNELS = ["webhook", "slack"] as const;
export type Channel = (typeof CHANNELS)[number];

export const DELIVERY_STATUSES = ["pending", "delivered", "failed"] as const;
export type DeliveryStatus = (typeof DELIVERY_STATUSES)[number];

export const DELIVERY_PAGE = 25;

export type Subscription = {
  key: string;
  channel: Channel;
  secret_id: string | null;
  secret: { id: string; name: string } | null;
  event_types: EventType[];
  environments: string[] | null;
  locale: string;
  disabled: boolean;
  signing: { fingerprint: string } | null;
  node_id: string;
  inherited: boolean;
  created_at: string;
  updated_at: string;
};

export type SubscriptionBody = {
  channel: Channel;
  secret_id: string | null;
  event_types: EventType[];
  environments: string[] | null;
  locale: string;
  disabled: boolean;
};

export type Delivery = {
  id: string;
  event_id: string;
  event_type: JournalEventType;
  event_node: { id: string; path?: string };
  subscription: string;
  channel: Channel;
  status: DeliveryStatus;
  attempts: number;
  next_attempt_at: string | null;
  last_http_status: number | null;
  last_error_code: string | null;
  last_error_message: string | null;
  created_at: string;
  delivered_at: string | null;
};

export type TestResult = { status: "delivered" | "failed"; http_status: number | null; error_code: string | null; duration_ms: number };

export type DeliveryParams = { subscription: string; status: DeliveryStatus | ""; page: number };

export function readDeliveryParams(get: (name: string) => string | null): DeliveryParams {
  const status = get("status") ?? "";
  const page = Number.parseInt(get("page") ?? "1", 10);
  return {
    subscription: get("subscription") ?? "",
    status: (DELIVERY_STATUSES as readonly string[]).includes(status) ? (status as DeliveryStatus) : "",
    page: Number.isFinite(page) && page > 0 ? page : 1,
  };
}

export function deliveryAddress(p: DeliveryParams): string {
  const q = new URLSearchParams();
  if (p.subscription) q.set("subscription", p.subscription);
  if (p.status) q.set("status", p.status);
  if (p.page > 1) q.set("page", String(p.page));
  return q.toString();
}

export function deliveryQuery(p: DeliveryParams): string {
  const q = new URLSearchParams();
  if (p.subscription) q.set("subscription", p.subscription);
  if (p.status) q.set("status", p.status);
  q.set("limit", String(DELIVERY_PAGE));
  q.set("offset", String((p.page - 1) * DELIVERY_PAGE));
  return q.toString();
}

export function bodyOf(s: Subscription, overrides: Partial<SubscriptionBody> = {}): SubscriptionBody {
  return {
    channel: s.channel,
    secret_id: s.secret_id,
    event_types: [...s.event_types],
    environments: s.environments ? [...s.environments] : null,
    locale: s.locale,
    disabled: s.disabled,
    ...overrides,
  };
}

export function environmentsOf(tags: string[]): string[] | null {
  const out = [...new Set(tags.map((t) => t.trim().toLowerCase()).filter(Boolean))];
  return out.length ? out : null;
}

const KEY_RE = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const ENV_RE = /^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$/;

export const validKey = (key: string) => KEY_RE.test(key.trim().toLowerCase());
export const validEnvironment = (env: string) => ENV_RE.test(env.trim().toLowerCase());

