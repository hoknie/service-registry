"use client";

import { Send } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import type { Page } from "@/lib/api";
import {
  DELIVERY_PAGE,
  DELIVERY_STATUSES,
  deliveryAddress,
  deliveryQuery,
  readDeliveryParams,
  type Delivery,
  type DeliveryParams,
  type DeliveryStatus,
  type Subscription,
} from "@/lib/notifications";
import { withScanAddress } from "@/lib/scans";
import { usePolledGet, useRefreshInterval } from "@/lib/useAutoRefresh";

import { catalogHref } from "../catalog/shared";
import { useUiText } from "../UiText";
import { AutoRefresh } from "../ui/AutoRefresh";
import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Field } from "../ui/Field";
import { Message } from "../ui/Message";
import { Pager } from "../ui/Pager";
import { Select } from "../ui/Select";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { codeText } from "./SubscriptionsTable";

type Labels = Messages["notify"];

const tones: Record<DeliveryStatus, Tone> = { pending: "amber", delivered: "signal", failed: "danger" };

type Props = { nodeId: string; locale: Locale; labels: Labels; subscriptions: Subscription[]; branch: string | null };

export function DeliveriesTable({ nodeId, locale, labels: t, subscriptions, branch }: Props) {
  const { errors, common } = useUiText();
  const router = useRouter();
  const search = useSearchParams();
  const params = readDeliveryParams((n) => search.get(n));
  const key = `/v1/catalog/nodes/${nodeId}/notification-deliveries?${deliveryQuery(params)}` as `/${string}`;
  const [interval, setIntervalSecs] = useRefreshInterval();
  const { page, failure, refreshing, refresh } = usePolledGet<Page<Delivery>>(key, interval, errors);
  const current = page?.key === key ? page.data : null;
  const error = failure?.key === key ? failure.text : null;
  const filtered = !!(params.subscription || params.status);
  const own = subscriptions.filter((s) => !s.inherited);

  const go = (next: Partial<DeliveryParams>) => {
    const address = deliveryAddress({ ...params, page: 1, ...next });
    router.replace(withScanAddress(catalogHref(locale, nodeId, "settings", branch, null, "notifications"), address), { scroll: false });
  };

  return (
    <section aria-label={t.journal.title} className="grid gap-4">
      <div className="flex flex-wrap items-end gap-3">
        <Field label={t.journal.subscription} className="min-w-0 flex-[1_1_12rem]">
          {(p) => (
            <Select {...p} value={params.subscription} onChange={(e) => go({ subscription: e.target.value })}>
              <option value="">{t.journal.subscriptionAny}</option>
              {own.map((s) => (
                <option key={s.key} value={s.key}>
                  {s.key}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Field label={t.journal.status} className="min-w-0 flex-[1_1_12rem]">
          {(p) => (
            <Select {...p} value={params.status} onChange={(e) => go({ status: e.target.value as DeliveryStatus | "" })}>
              <option value="">{t.journal.statusAny}</option>
              {DELIVERY_STATUSES.map((s) => (
                <option key={s} value={s}>
                  {t.statuses[s]}
                </option>
              ))}
            </Select>
          )}
        </Field>
        {filtered && (
          <Button variant="ghost" onClick={() => go({ subscription: "", status: "" })}>
            {t.journal.reset}
          </Button>
        )}
        <div className="ml-auto">
          <AutoRefresh labels={t.refresh} value={interval} onChange={setIntervalSecs} busy={refreshing} onRefresh={refresh} />
        </div>
      </div>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!current && !error && <SkeletonTable rows={4} label={common.loading} />}
      {current && current.items.length === 0 && <EmptyState icon={Send} title={filtered ? t.journal.emptyFiltered : t.journal.empty} />}
      {current && current.items.length > 0 && (
        <>
          <div className="@container min-w-0">
            <Table>
              <thead>
                <tr>
                  <Th>{t.journal.columns.time}</Th>
                  <Th>{t.journal.columns.event}</Th>
                  <Th className="hidden @2xl:table-cell">{t.journal.columns.node}</Th>
                  <Th className="hidden @xl:table-cell">{t.journal.columns.subscription}</Th>
                  <Th>{t.journal.columns.status}</Th>
                  <Th numeric className="hidden @lg:table-cell">
                    {t.journal.columns.attempts}
                  </Th>
                  <Th numeric className="hidden @lg:table-cell">
                    {t.journal.columns.http}
                  </Th>
                  <Th className="hidden @3xl:table-cell">{t.journal.columns.error}</Th>
                </tr>
              </thead>
              <tbody>
                {current.items.map((d) => (
                  <Tr key={d.id}>
                    <Td>
                      <time dateTime={d.created_at} title={when(d.created_at, locale, "")} className="whitespace-nowrap">
                        {ago(d.created_at, locale, "")}
                      </time>
                    </Td>
                    <Td>{t.events[d.event_type]}</Td>
                    <Td className="hidden @2xl:table-cell">
                      {d.event_node.path ? (
                        <Link href={catalogHref(locale, d.event_node.id)} className="font-mono text-xs break-all text-signal hover:underline">
                          {d.event_node.path}
                        </Link>
                      ) : (
                        <span className="text-xs text-muted">{t.journal.hiddenNode}</span>
                      )}
                    </Td>
                    <Td className="hidden font-mono @xl:table-cell">{d.subscription}</Td>
                    <Td>
                      <Badge tone={tones[d.status]}>{t.statuses[d.status]}</Badge>
                      {d.status === "pending" && d.next_attempt_at && (
                        <span className="block text-xs text-muted">{format(t.journal.nextAttempt, { when: ago(d.next_attempt_at, locale, "") })}</span>
                      )}
                    </Td>
                    <Td numeric className="hidden @lg:table-cell">
                      {d.attempts}
                    </Td>
                    <Td numeric className="hidden @lg:table-cell">
                      {d.last_http_status ?? "—"}
                    </Td>
                    <Td className="hidden max-w-[20rem] text-xs text-ink-2 @3xl:table-cell">
                      {d.last_error_code ? codeText(t, errors.unknown, d.last_error_code) : "—"}
                    </Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          </div>
          <Pager
            offset={(params.page - 1) * DELIVERY_PAGE}
            shown={current.items.length}
            total={current.total}
            size={DELIVERY_PAGE}
            labels={t.pager}
            onChange={(offset) => go({ page: Math.floor(offset / DELIVERY_PAGE) + 1 })}
          />
        </>
      )}
    </section>
  );
}
