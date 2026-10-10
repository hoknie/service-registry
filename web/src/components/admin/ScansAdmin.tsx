"use client";

import { ScanSearch } from "lucide-react";
import { useMemo } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { index, path } from "@/lib/catalogNav";
import { useCatalogTree } from "@/lib/catalogTree";
import { withScanAddress } from "@/lib/scans";

import { ScansTable } from "../knowledge/ScansTable";
import { PageHeader } from "../ui/Panel";

export function ScansAdmin({ locale, labels: t }: { locale: Locale; labels: Messages["scans"] }) {
  const tree = useCatalogTree();
  const projects = useMemo(() => {
    if (!tree.tree) return [];
    const ix = index(tree.tree.nodes);
    return tree.tree.nodes
      .filter((n) => n.kind === "project")
      .map((n) => ({ value: n.id, label: n.name, detail: path(ix, n.id).map((p) => p.slug).join("/") }))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [tree.tree]);
  return (
    <div className="grid gap-6">
      <PageHeader title={t.title} lead={t.lead} glyph={ScanSearch} />
      <ScansTable
        locale={locale}
        labels={t}
        url="/v1/knowledge/scans"
        hrefOf={(address) => withScanAddress(`/${locale}/admin/scans`, address)}
        projects={projects}
      />
    </div>
  );
}
