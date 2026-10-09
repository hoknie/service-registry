"use client";

import { FileText, Pencil } from "lucide-react";
import Link from "next/link";

import type { Locale } from "@/i18n/config";
import type { CatalogNode } from "@/lib/api";

import { EmptyState } from "../ui/EmptyState";
import { Panel } from "../ui/Panel";
import { Markdown } from "./Markdown";
import { catalogHref, type CatalogLabels } from "./shared";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; canWrite: boolean };

export function DescriptionTab({ node, locale, labels, canWrite }: Props) {
  const t = labels.about;
  const description = node.description?.trim() ?? "";
  if (!description) {
    return (
      <EmptyState
        icon={FileText}
        title={t.empty}
        action={
          canWrite ? (
            <Link
              href={catalogHref(locale, node.id, "settings", null, null, "general")}
              className="inline-flex items-center gap-1.5 text-sm font-medium text-signal hover:underline"
            >
              <Pencil aria-hidden="true" className="size-4" />
              {t.add}
            </Link>
          ) : undefined
        }
      />
    );
  }
  return (
    <Panel title={t.title}>
      <Markdown source={description} />
    </Panel>
  );
}
