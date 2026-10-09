"use client";

import { ArrowRight, Box, Building, Folder, FolderTree, Rocket, UserRound, UsersRound } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { apiGet, errorCode, type CatalogNode, type Items, type NodeKind, type ServiceEnvironment } from "@/lib/api";
import { useCatalogTree } from "@/lib/catalogTree";

import { ApiStatus } from "./ApiStatus";
import { catalogHref } from "./catalog/shared";
import { KindIcon } from "./catalog/KindIcon";
import { useSession } from "./SessionProvider";
import { useUiText } from "./UiText";
import { Badge } from "./ui/Badge";
import { EmptyState } from "./ui/EmptyState";
import { Message } from "./ui/Message";
import { Panel } from "./ui/Panel";
import { SkeletonRows } from "./ui/Skeleton";
import { StatCard } from "./ui/StatCard";

const kindIcons = { organization: Building, folder: Folder, project: Box } as const;
const kindTones = { organization: "org", folder: "folder", project: "project" } as const;

const RECENT_PROJECTS = 6;
const RECENT_ROWS = 8;

type Recent = ServiceEnvironment & { occurred_at: string; project: CatalogNode };
type Labels = Messages["home"];

export function Dashboard({ locale, labels }: { locale: Locale; labels: Labels }) {
  const { errors, common } = useUiText();
  const { session } = useSession();
  const catalog = useCatalogTree();
  const tree = catalog.tree;
  const [recent, setRecent] = useState<Recent[] | null>(null);
  const [failure, setError] = useState<string | null>(null);
  const error = failure ?? (catalog.error ? errorText(errors, errorCode(catalog.error)) : null);

  useEffect(() => {
    if (!tree) return;
    let alive = true;
    (async () => {
      try {
        const projects = tree.nodes
          .filter((n) => n.kind === "project" && n.access === "read")
          .sort((a, b) => (b.updated_at ?? "").localeCompare(a.updated_at ?? ""))
          .slice(0, RECENT_PROJECTS);
        const lists = await Promise.all(
          projects.map((p) =>
            apiGet<Items<ServiceEnvironment>>(`/v1/catalog/nodes/${p.id}/environments`)
              .then((r) => r.items.flatMap((e) => (e.occurred_at ? [{ ...e, occurred_at: e.occurred_at, project: p }] : [])))
              .catch(() => [] as Recent[]),
          ),
        );
        if (!alive) return;
        setRecent(
          lists
            .flat()
            .sort((a, b) => b.occurred_at.localeCompare(a.occurred_at))
            .slice(0, RECENT_ROWS),
        );
      } catch (e) {
        if (alive) setError(errorText(errors, errorCode(e)));
      }
    })();
    return () => {
      alive = false;
    };
  }, [tree, errors]);

  const count = (kind: NodeKind) => tree?.nodes.filter((n) => n.kind === kind).length ?? 0;
  const superadmin = session.status === "signed-in" && session.user.is_superadmin;
  const s = labels.summary;

  const links = [
    { href: `/${locale}/catalog`, icon: FolderTree, title: labels.links.catalog, text: labels.links.catalogText },
    { href: `/${locale}/account`, icon: UserRound, title: labels.links.account, text: labels.links.accountText },
    ...(superadmin
      ? [
          { href: `/${locale}/admin/users`, icon: UserRound, title: labels.links.users, text: labels.links.usersText },
          { href: `/${locale}/admin/groups`, icon: UsersRound, title: labels.links.groups, text: labels.links.groupsText },
        ]
      : []),
  ];

  return (
    <div className="grid gap-6">
      {error && <Message note={{ kind: "error", text: error }} />}

      <section aria-label={s.title} className="grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-4">
        {(["organization", "folder", "project"] as const).map((kind) => (
          <StatCard
            key={kind}
            icon={kindIcons[kind]}
            tone={kindTones[kind]}
            title={s[kind]}
            href={`/${locale}/catalog`}
            loading={!tree && !error}
            className="min-h-32"
            value={
              tree ? (
                <>
                  {count(kind)}
                  {tree.truncated && <span className="text-muted">+</span>}
                </>
              ) : undefined
            }
          />
        ))}
        <ApiStatus labels={labels.apiStatus} />
      </section>
      {tree?.truncated && <p className="-mt-3 text-sm text-muted">{s.truncated}</p>}

      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
        <Panel icon={Rocket} title={labels.recent.title} description={labels.recent.lead} bodyClassName="p-0 sm:p-0">
          {!recent && !error && <SkeletonRows rows={5} label={common.loading} className="p-5" />}
          {recent && recent.length === 0 && (
            <EmptyState icon={Rocket} title={labels.recent.empty} text={labels.recent.emptyText} className="m-5" />
          )}
          {recent && recent.length > 0 && (
            <ul className="divide-y divide-line">
              {recent.map((r) => (
                <li key={`${r.project.id}/${r.service}/${r.environment}`}>
                  <Link
                    href={catalogHref(locale, r.project.id, "deployments")}
                    className="motion-control flex flex-wrap items-center gap-x-4 gap-y-1 px-5 py-3 hover:bg-surface-2"
                  >
                    <KindIcon kind="project" tile className="size-8 rounded-lg [&>svg]:size-4" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate font-medium text-ink">{r.service}</span>
                      <span className="block truncate text-xs text-muted">{r.project.name}</span>
                    </span>
                    <Badge tone="signal" dot>
                      {r.environment}
                    </Badge>
                    <code className="rounded bg-surface-3 px-1.5 py-0.5 text-xs text-ink">{r.version}</code>
                    <time dateTime={r.occurred_at} title={when(r.occurred_at, locale, "")} className="w-28 text-right text-xs text-muted tabular-nums">
                      {ago(r.occurred_at, locale, "")}
                    </time>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Panel>

        <Panel icon={ArrowRight} title={labels.links.title} bodyClassName="p-2 sm:p-2">
          <ul className="grid gap-1">
            {links.map((l) => (
              <li key={l.href}>
                <Link href={l.href} className="group motion-control flex items-start gap-3 rounded-lg p-3 hover:bg-surface-2">
                  <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-lg bg-surface-3 text-muted group-hover:bg-signal-soft group-hover:text-signal">
                    <l.icon className="size-4" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block font-medium text-ink">{l.title}</span>
                    <span className="block text-sm text-muted">{l.text}</span>
                  </span>
                  <ArrowRight aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-muted opacity-0 transition-opacity group-hover:opacity-100" />
                </Link>
              </li>
            ))}
          </ul>
        </Panel>
      </div>
    </div>
  );
}
