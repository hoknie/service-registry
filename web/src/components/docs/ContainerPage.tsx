"use client";

import { FileText, FolderTree } from "lucide-react";

import type { Locale } from "@/i18n/config";
import type { DocSpace } from "@/lib/api";
import { cn } from "@/lib/cn";
import { docHref } from "@/lib/docPath";

import { KindIcon } from "../catalog/KindIcon";
import { Markdown } from "../catalog/Markdown";
import { EmptyState } from "../ui/EmptyState";
import { PageHeader } from "../ui/Panel";
import type { DocLabels } from "./DocSpace";
import { DocLink } from "./DocLink";

type Props = { locale: Locale; labels: DocLabels; space: DocSpace; nodePath: string[] };

export function ContainerPage({ labels, space, nodePath }: Props) {
  const t = labels.space;
  const node = space.node;
  return (
    <div className="grid gap-6">
      {node ? (
        <header className="flex items-start gap-3">
          <KindIcon kind={node.kind} tile className="size-11 rounded-xl [&>svg]:size-5" />
          <div className="min-w-0">
            <p className="text-xs font-medium tracking-wide text-muted uppercase">{t.kinds[node.kind]}</p>
            <h1 className="text-xl font-semibold text-ink">{node.name}</h1>
          </div>
        </header>
      ) : (
        <PageHeader glyph={FolderTree} title={t.title} lead={t.lead} />
      )}
      {node?.description?.trim() && <Markdown source={node.description} />}
      {space.children.length === 0 ? (
        <EmptyState icon={FileText} title={t.empty} />
      ) : (
        <section aria-label={node ? t.contents : t.spaces}>
          <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {space.children.map((c) => {
              const muted = c.has_docs === false;
              return (
                <li key={c.id}>
                  <DocLink
                    href={docHref([...nodePath, c.slug])}
                    className={cn(
                      "motion-control flex h-full items-start gap-3 rounded-card border border-line p-4 hover:border-signal hover:shadow-elev-2",
                      muted ? "border-dashed bg-surface-2" : "bg-surface shadow-elev-1",
                    )}
                  >
                    <KindIcon kind={c.kind} tile className="size-9 rounded-lg [&>svg]:size-4" />
                    <span className="grid min-w-0 gap-0.5">
                      <span className="truncate font-medium text-ink">{c.name}</span>
                      <span className="text-xs text-muted">{muted ? t.notCollected : t.kinds[c.kind]}</span>
                    </span>
                  </DocLink>
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </div>
  );
}
