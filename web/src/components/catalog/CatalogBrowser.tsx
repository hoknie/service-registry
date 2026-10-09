"use client";

import { useSearchParams } from "next/navigation";
import { useCallback } from "react";

import type { Locale } from "@/i18n/config";
import { reloadCatalogTree } from "@/lib/catalogTree";

import { NodeView } from "./NodeView";
import type { CatalogLabels } from "./shared";

type Props = { locale: Locale; labels: CatalogLabels };

export function CatalogBrowser({ locale, labels }: Props) {
  const params = useSearchParams();
  const nodeId = params.get("node");
  const tab = params.get("tab");
  const branch = params.get("branch");
  const doc = params.get("doc");
  const changed = useCallback(() => reloadCatalogTree(), []);

  return (
    <section className="min-w-0">
      <NodeView key={nodeId ?? "top"} id={nodeId} tab={tab} branch={branch} doc={doc} locale={locale} labels={labels} onChanged={changed} />
    </section>
  );
}
