"use client";

import type { Locale } from "@/i18n/config";
import { withScanAddress } from "@/lib/scans";

import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { Panel } from "../ui/Panel";
import { ScansTable } from "./ScansTable";

type Props = { projectId: string; branch: string | null; locale: Locale; labels: CatalogLabels };

export function ProjectScans({ projectId, branch, locale, labels }: Props) {
  const t = labels.scans;
  return (
    <Panel title={t.title} description={t.projectLead}>
      <ScansTable
        locale={locale}
        labels={t}
        url={`/v1/catalog/nodes/${projectId}/knowledge/scans`}
        hrefOf={(address) => withScanAddress(catalogHref(locale, projectId, "settings", branch, null, "scans"), address)}
      />
    </Panel>
  );
}
