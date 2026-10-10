"use client";

import { useSearchParams } from "next/navigation";

import type { Locale } from "@/i18n/config";
import { catalogChanged } from "@/lib/catalogEvents";

import { NodeView } from "./NodeView";
import type { CatalogLabels } from "./shared";

type Props = { locale: Locale; labels: CatalogLabels };

export function CatalogBrowser({ locale, labels }: Props) {
  const params = useSearchParams();
  const nodeId = params.get("node");
  const tab = params.get("tab");
  const section = params.get("section");
  const branch = params.get("branch");
  const doc = params.get("doc");

  return (
    <section className="min-w-0">
      <NodeView key={nodeId ?? "top"} id={nodeId} tab={tab} section={section} branch={branch} doc={doc} locale={locale} labels={labels} onChanged={catalogChanged} />
    </section>
  );
}
