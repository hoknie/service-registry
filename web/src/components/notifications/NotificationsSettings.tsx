"use client";

import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { errorCode, type CatalogNode } from "@/lib/api";
import type { Subscription } from "@/lib/notifications";
import { listSubscriptions } from "@/lib/notificationsApi";

import type { CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonRows } from "../ui/Skeleton";
import { DeliveriesTable } from "./DeliveriesTable";
import { SubscriptionsTable } from "./SubscriptionsTable";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; branch: string | null };

export function NotificationsSettings({ node, locale, labels, branch }: Props) {
  const t = labels.notify;
  const { errors, common } = useUiText();
  const [items, setItems] = useState<Subscription[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setItems(await listSubscriptions(node.id));
      setError(null);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  }, [node.id, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  return (
    <div className="grid gap-6">
      <Panel title={t.title} description={t.lead}>
        {error && <Message note={{ kind: "error", text: error }} />}
        {!items && !error && <SkeletonRows rows={3} label={common.loading} />}
        {items && <SubscriptionsTable node={node} locale={locale} labels={t} items={items} reload={() => void load()} />}
      </Panel>
      <Panel title={t.journal.title} description={t.journal.lead}>
        <DeliveriesTable nodeId={node.id} locale={locale} labels={t} subscriptions={items ?? []} branch={branch} />
      </Panel>
    </div>
  );
}
