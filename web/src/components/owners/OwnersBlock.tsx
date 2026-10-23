"use client";

import { User, UsersRound } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { CatalogNode, Owner, OwnerSet } from "@/lib/api";
import { useCatalogVersion } from "@/lib/catalogEvents";
import { getOwners } from "@/lib/ownersApi";

import { Badge } from "../ui/Badge";
import { Panel } from "../ui/Panel";
import { Skeleton } from "../ui/Skeleton";
import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { GroupContactsView } from "./GroupContactsView";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels };

export function OwnersBlock({ node, locale, labels }: Props) {
  const t = labels.owners;
  const [set, setSet] = useState<OwnerSet | null>(null);
  const version = useCatalogVersion();

  useEffect(() => {
    const abort = new AbortController();
    getOwners(node.id, abort.signal)
      .then(setSet)
      .catch(() => !abort.signal.aborted && setSet({ items: [], from: null, own: false, none: false }));
    return () => abort.abort();
  }, [node.id, version]);

  if (!set) {
    return (
      <Panel title={t.title} icon={UsersRound}>
        <Skeleton className="h-6 w-48" />
      </Panel>
    );
  }
  if (set.items.length === 0) return null;
  const from = set.from && !set.own ? [...(node.path ?? [])].find((p) => p.id === set.from) : undefined;

  return (
    <Panel
      title={t.title}
      icon={UsersRound}
      actions={
        from && (
          <Link href={catalogHref(locale, from.id)} className="text-sm text-signal hover:underline">
            {format(t.inheritedFrom, { name: from.name })}
          </Link>
        )
      }
    >
      <ul className="grid gap-3 sm:grid-cols-2">
        {set.items.map((o, i) => (
          <OwnerCard key={`${o.subject_kind}:${o.subject_id}`} owner={o} primary={i === 0} labels={labels} />
        ))}
      </ul>
    </Panel>
  );
}

function OwnerCard({ owner, primary, labels }: { owner: Owner; primary: boolean; labels: CatalogLabels }) {
  const t = labels.owners;
  const Icon = owner.subject_kind === "group" ? UsersRound : User;
  return (
    <li className="grid min-w-0 content-start gap-2 rounded-lg border border-line p-3">
      <span className="flex min-w-0 items-center gap-2">
        <Icon aria-label={owner.subject_kind === "group" ? t.group : t.user} className="size-4 shrink-0 text-muted" />
        <span className="min-w-0 truncate font-medium text-ink">{owner.name}</span>
        {primary && <Badge tone="signal">{t.primary}</Badge>}
        {owner.status === "disabled" && <Badge tone="outline">{t.disabled}</Badge>}
      </span>
      {owner.email && (
        <a href={`mailto:${owner.email}`} className="truncate text-sm text-signal hover:underline">
          {owner.email}
        </a>
      )}
      <GroupContactsView contacts={owner.contacts} onCallUrl={owner.on_call_url} labels={t} />
    </li>
  );
}
