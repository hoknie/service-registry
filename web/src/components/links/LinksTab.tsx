"use client";

import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { apiGet, type CatalogNode, type Items, type LinkKind } from "@/lib/api";

import type { CatalogLabels } from "../catalog/shared";
import { LinksPanel } from "./LinksPanel";
import { TemplatesEditor } from "./TemplatesEditor";
import { VarsEditor } from "./VarsEditor";

type Props = { node: CatalogNode; branch: string | null; locale: Locale; labels: CatalogLabels; canWrite: boolean };

export function LinksTab({ node, branch, locale, labels, canWrite }: Props) {
  const [kinds, setKinds] = useState<LinkKind[]>([]);
  const [version, setVersion] = useState(0);

  useEffect(() => {
    apiGet<Items<LinkKind>>("/v1/link-kinds")
      .then((r) => setKinds(r.items))
      .catch(() => setKinds([]));
  }, []);

  const changed = () => setVersion((v) => v + 1);

  return (
    <div className="grid gap-6">
      {node.kind === "project" && (
        <LinksPanel key={`${branch ?? ""}:${version}`} projectId={node.id} branch={branch} kinds={kinds} locale={locale} labels={labels} />
      )}
      <TemplatesEditor node={node} kinds={kinds} locale={locale} labels={labels} canWrite={canWrite} onChanged={changed} />
      <VarsEditor node={node} locale={locale} labels={labels} canWrite={canWrite} onChanged={changed} />
    </div>
  );
}
