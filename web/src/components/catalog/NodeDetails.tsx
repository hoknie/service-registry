import type { Locale } from "@/i18n/config";
import { ago, when } from "@/i18n/time";
import type { CatalogNode } from "@/lib/api";
import type { ReactNode } from "react";

import { LinksDetails } from "../links/LinksDetails";
import { OwnersBlock } from "../owners/OwnersBlock";
import { PublicAddresses } from "../publication/PublicAddresses";
import { Panel } from "../ui/Panel";
import type { CatalogLabels } from "./shared";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; branch: string | null; canWrite: boolean; children?: ReactNode };

export function NodeDetails({ node, locale, labels: t, branch, canWrite, children }: Props) {
  return (
    <div className="grid gap-6">
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
    <OwnersBlock node={node} locale={locale} labels={t} />
    <PublicAddresses node={node} labels={t} />
    {children}
    <Panel>
      <LinksDetails node={node} locale={locale} labels={t} branch={branch} canWrite={canWrite} />
    </Panel>
    </div>
  );
}
