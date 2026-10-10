import type { Locale } from "@/i18n/config";
import { ago, when } from "@/i18n/time";
import type { CatalogNode } from "@/lib/api";

import { Panel } from "../ui/Panel";
import type { CatalogLabels } from "./shared";

export function NodeDetails({ node, locale, labels: t }: { node: CatalogNode; locale: Locale; labels: CatalogLabels }) {
  return (
    <Panel title={t.node.details}>
      <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-2.5 text-sm">
        <dt className="text-muted">{t.node.kind}</dt>
        <dd className="text-ink">{t.kinds[node.kind]}</dd>
        <dt className="text-muted">{t.node.slug}</dt>
        <dd>
          <code className="text-ink">{node.slug}</code>
        </dd>
        <dt className="text-muted">{t.node.created}</dt>
        <dd className="text-ink" title={when(node.created_at, locale, "")}>
          {ago(node.created_at, locale, t.node.none)}
        </dd>
        <dt className="text-muted">{t.node.updated}</dt>
        <dd className="text-ink" title={when(node.updated_at, locale, "")}>
          {ago(node.updated_at, locale, t.node.none)}
        </dd>
      </dl>
    </Panel>
  );
}
