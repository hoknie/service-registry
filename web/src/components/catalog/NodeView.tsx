"use client";

import { Activity, BookOpen, ExternalLink, FileText, FolderTree, GitBranch, LayoutGrid, Plus, Rocket, Settings } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { ago, when } from "@/i18n/time";
import { busy, isSummary, useActivity } from "@/lib/activity";
import { apiGet, errorCode, type CatalogNode, type CatalogTable, type Items, type NodeActivity, type ProcessKind, type ProjectLink } from "@/lib/api";
import { useChildrenView } from "@/lib/childrenView";
import { resolveTab, sectionsOf, tabsOf, type NodeTab, type Section } from "@/lib/nodeSettings";

import { ForgeTab } from "../forge/ForgeTab";
import { DocsTab } from "../knowledge/DocsTab";
import { DocsSettingsTab } from "../knowledge/DocsSettingsTab";
import { LinksTab } from "../links/LinksTab";
import { RepositoryPanel } from "../forge/RepositoryPanel";
import { LinkIconRow } from "../links/LinkIcons";
import { useSession } from "../SessionProvider";
import { Crumbs } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Badge, LabelChip } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Card } from "../ui/Card";
import { Input } from "../ui/Field";
import { NodeHero } from "../ui/NodeHero";
import { Segmented } from "../ui/Segmented";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { PageHeader, Panel } from "../ui/Panel";
import { Pager } from "../ui/Pager";
import { Skeleton, SkeletonRows } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../ui/Tabs";
import { AccessTab } from "./AccessTab";
import { ProcessBadges, SummaryBadges } from "./ActivityBadges";
import { BranchesTab } from "./BranchesTab";
import { BranchPicker } from "./BranchPicker";
import { DeploymentsTab } from "./DeploymentsTab";
import { EventsTab } from "./EventsTab";
import { IngestHowTo } from "./IngestHowTo";
import { KindIcon } from "./KindIcon";
import { DescriptionTab } from "./DescriptionTab";
import { Markdown } from "./Markdown";
import { CreateDialog } from "./NodeForms";
import { NodeSettings } from "./NodeSettings";
import { ProjectKeys } from "./ProjectKeys";
import { ProjectSummary } from "./ProjectSummary";
import { SettingsGeneral } from "./SettingsGeneral";
import { TreeTable } from "./TreeTable";
import { catalogHref, childKinds, FORGE_NAMES, type CatalogLabels } from "./shared";

const PAGE = 50;

const tabIcons = {
  overview: LayoutGrid,
  about: FileText,
  deployments: Rocket,
  events: Activity,
  docs: BookOpen,
  settings: Settings,
} as const;

type Props = {
  id: string | null;
  tab: string | null;
  section: string | null;
  branch: string | null;
  doc: string | null;
  locale: Locale;
  labels: CatalogLabels;
  onChanged: () => void;
};

type Dialogs = "create" | null;

export function NodeView({ id, tab: tabParam, section: sectionParam, branch: branchParam, doc, locale, labels, onChanged }: Props) {
  const { errors, common } = useUiText();
  const { session } = useSession();
  const router = useRouter();
  const [node, setNode] = useState<CatalogNode | null>(null);
  const [children, setChildren] = useState<CatalogTable | null>(null);
  const [docsReload, setDocsReload] = useState(0);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [dialog, setDialog] = useState<Dialogs>(null);
  const [childFilter, setChildFilter] = useState("");
  const [view, setView] = useChildrenView();

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const loadChildren = useCallback(async () => {
    const parent = id ? `parent=${id}&` : "";
    setChildren(await apiGet<CatalogTable>(`/v1/catalog/table?${parent}limit=${PAGE}&offset=${offset}`));
  }, [id, offset]);

  const load = useCallback(async () => {
    try {
      if (id) setNode(await apiGet<CatalogNode>(`/v1/catalog/nodes/${id}`));
      await loadChildren();
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [id, loadChildren, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const loadActivity = useCallback((nodeId: string) => apiGet<NodeActivity>(`/v1/catalog/nodes/${nodeId}/activity`), []);
  const onSettled = useCallback((kinds: ProcessKind[]) => {
    if (kinds.includes("collect")) setDocsReload((n) => n + 1);
  }, []);
  const { activity, refresh: refreshActivity } = useActivity(id, loadActivity, onSettled);
  const childrenBusy = useRef(false);
  const [heroLinks, setHeroLinks] = useState<ProjectLink[]>([]);
  const projectId = node?.kind === "project" && node.access === "read" ? node.id : null;

  useEffect(() => {
    if (!projectId) return;
    let live = true;
    const q = branchParam ? `?branch=${encodeURIComponent(branchParam)}` : "";
    apiGet<Items<ProjectLink>>(`/v1/catalog/nodes/${projectId}/links${q}` as `/${string}`)
      .then((r) => {
        if (!live) return;
        const first = new Map<string, ProjectLink>();
        for (const l of r.items) {
          const seen = first.get(l.link_key);
          if (!seen || (!seen.url && l.url)) first.set(l.link_key, l);
        }
        setHeroLinks([...first.values()]);
      })
      .catch(() => live && setHeroLinks([]));
    return () => {
      live = false;
    };
  }, [projectId, branchParam]);

  useEffect(() => {
    if (!activity || node?.kind === "project" || view === "tree") return;
    const now = busy(activity);
    if (now || childrenBusy.current) void loadChildren().catch(() => undefined);
    childrenBusy.current = now;
  }, [activity, node?.kind, view, loadChildren]);

  const resolved = resolveTab(node, tabParam, sectionParam);
  const redirect = resolved.redirect;
  useEffect(() => {
    if (node && redirect) {
      router.replace(catalogHref(locale, node.id, redirect.tab, branchParam, null, redirect.section), { scroll: false });
    }
  }, [node, redirect, router, locale, branchParam]);

  const changed = () => {
    void load();
    onChanged();
  };

  const t = labels;
  const rootCrumb = { label: t.root, href: catalogHref(locale, null), node: "root" };

  if (error && !children) {
    return (
      <>
        <Crumbs items={[rootCrumb]} />
        <EmptyState icon={FolderTree} title={error} className="mt-4" />
      </>
    );
  }
  if (!children || (id && !node)) {
    return (
      <div className="grid gap-6">
        <div className="flex items-center gap-3">
          <Skeleton className="size-10" />
          <Skeleton className="h-7 w-56" />
        </div>
        <SkeletonRows rows={6} label={common.loading} />
      </div>
    );
  }

  const superadmin = session.status === "signed-in" && session.user.is_superadmin;
  const can = (p: string) => (node ? (node.permissions ?? []).includes(p) : superadmin);
  const readable = !node || node.access === "read";
  const kinds = childKinds(node ? node.kind : null);
  const project = node?.kind === "project";
  const canWrite = can("catalog.write");
  const canCreate = canWrite && kinds.length > 0;
  const managed = !!node?.managed;
  const canMove = !!node && !managed && ((node.permissions ?? []).includes("catalog.write") || superadmin);
  const canDelete = !!node && !managed && (node.permissions ?? []).includes("catalog.write");

  const tabs = tabsOf(node);
  const tab: NodeTab = resolved.tab;
  const sections = sectionsOf(node);
  const section: Section = resolved.section ?? "general";
  const selectTab = (next: string) => {
    if (node) router.replace(catalogHref(locale, node.id, next, branchParam), { scroll: false });
  };
  const branch = branchParam ?? node?.default_branch ?? null;
  const selectBranch = (name: string) => {
    if (node) router.replace(catalogHref(locale, node.id, tab, name === node.default_branch ? null : name), { scroll: false });
  };

  const addButton = canCreate && (
    <Button variant="primary" onClick={() => setDialog("create")}>
      <Plus aria-hidden="true" />
      {t.create.title}
    </Button>
  );

  const actions = node && addButton;

  const labelEntries = Object.entries(node?.labels ?? {});

  const filtered = children.items.filter((c) => {
    const q = childFilter.trim().toLowerCase();
    return !q || c.name.toLowerCase().includes(q) || c.slug.toLowerCase().includes(q);
  });

  const childActivity = (c: CatalogTable["items"][number]) =>
    isSummary(c.activity) ? (
      <SummaryBadges summary={c.activity} labels={t.activity} />
    ) : (
      <ProcessBadges processes={c.activity} labels={t.activity} locale={locale} compact />
    );

  const childCards = (
    <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {filtered.map((c) => {
        const labels = Object.entries(c.labels ?? {});
        return (
          <li key={c.id} className="min-w-0">
            <Card href={catalogHref(locale, c.id)} className="h-full">
              <span className="flex items-start gap-3">
                <KindIcon kind={c.kind} tile className="size-9" />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-ink group-hover:text-signal">{c.name}</span>
                  <span className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted">
                    <span>{t.kinds[c.kind]}</span>
                    <code className="truncate">{c.slug}</code>
                  </span>
                </span>
              </span>
              {c.access === "navigate" && (
                <Badge tone="outline" className="mt-3">
                  {t.navigateOnly}
                </Badge>
              )}
              <span className="mt-3 block empty:hidden">{childActivity(c)}</span>
              {c.links.length > 0 && <LinkIconRow links={c.links} locale={locale} labels={t.linkIcons} max={6} inert className="mt-3" />}
              {labels.length > 0 && (
                <span className="mt-3 flex flex-wrap gap-1">
                  {labels.slice(0, 4).map(([k, v]) => (
                    <LabelChip key={k} name={k} value={v} />
                  ))}
                  {labels.length > 4 && <Badge tone="outline">+{labels.length - 4}</Badge>}
                </span>
              )}
            </Card>
          </li>
        );
      })}
    </ul>
  );

  const childTable = (
    <Table>
      <thead>
        <tr>
          <Th>{t.children.name}</Th>
          <Th>{t.children.kind}</Th>
          <Th>{t.children.slug}</Th>
          <Th className="hidden md:table-cell">{t.node.labels}</Th>
          <Th>{t.tree.activity}</Th>
        </tr>
      </thead>
      <tbody>
        {filtered.map((c) => (
          <Tr key={c.id}>
            <Td>
              <Link href={catalogHref(locale, c.id)} className="flex items-center gap-2.5 font-medium text-ink hover:underline">
                <KindIcon kind={c.kind} />
                {c.name}
              </Link>
              {c.access === "navigate" && (
                <Badge tone="outline" className="mt-1">
                  {t.navigateOnly}
                </Badge>
              )}
            </Td>
            <Td className="text-muted">{t.kinds[c.kind]}</Td>
            <Td>
              <code className="text-xs text-ink-2">{c.slug}</code>
            </Td>
            <Td className="hidden md:table-cell">
              <span className="flex flex-wrap gap-1">
                {Object.entries(c.labels ?? {})
                  .slice(0, 4)
                  .map(([k, v]) => (
                    <LabelChip key={k} name={k} value={v} />
                  ))}
              </span>
            </Td>
            <Td>
              <span className="grid gap-1.5">
                {childActivity(c)}
                {c.links.length > 0 && <LinkIconRow links={c.links} locale={locale} labels={t.linkIcons} max={6} />}
              </span>
            </Td>
          </Tr>
        ))}
      </tbody>
    </Table>
  );

  const childrenList = (
    <section aria-labelledby="children-title" className="grid gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <h2 id="children-title" className="flex items-center gap-2">
          {t.children.title}
          <Badge>{children.total}</Badge>
        </h2>
        {children.items.length > 0 && (
          <div className="ml-auto flex flex-wrap items-center gap-2">
            {view !== "tree" && (
              <Input
                type="search"
                value={childFilter}
                onChange={(e) => setChildFilter(e.target.value)}
                placeholder={t.children.filter}
                aria-label={t.children.filter}
                className="w-56 max-w-full"
              />
            )}
            <Segmented
              label={t.children.view}
              value={view}
              onChange={(v) => setView(v === "table" || v === "tree" ? v : "cards")}
              options={[
                { value: "cards", label: t.children.cards },
                { value: "table", label: t.children.table },
                { value: "tree", label: t.children.tree },
              ]}
            />
          </div>
        )}
      </div>
      {children.items.length > 0 && view === "tree" ? (
        <TreeTable rootId={id} locale={locale} labels={t} />
      ) : children.items.length === 0 ? (
        <EmptyState
          icon={FolderTree}
          title={t.children.empty}
          text={canCreate ? t.children.emptyText : undefined}
          action={canCreate ? addButton : undefined}
        />
      ) : (
        <>
          {filtered.length === 0 ? (
            <p className="text-sm text-muted">{t.children.noMatch}</p>
          ) : view === "table" ? (
            childTable
          ) : (
            childCards
          )}
          <Pager offset={offset} shown={children.items.length} total={children.total} size={PAGE} labels={t.pager} onChange={setOffset} />
        </>
      )}
    </section>
  );

  const overview = node && (
    <div className="grid gap-6">
      {!project && readable && node.description?.trim() && (
        <Panel title={t.about.title}>
          <Markdown source={node.description} />
        </Panel>
      )}
      {project && readable && <ProjectSummary projectId={node.id} locale={locale} labels={t} />}
      {readable && (
        <div className="grid items-start gap-4 md:grid-cols-2">
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
          {project && (
            <Panel title={t.repo.title}>
              <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-2.5 text-sm">
                <dt className="text-muted">{t.repo.forge}</dt>
                <dd className="text-ink">{node.forge ? FORGE_NAMES[node.forge] : t.repo.noForge}</dd>
                <dt className="text-muted">{t.repo.url}</dt>
                <dd className="min-w-0">
                  {node.repo_url ? (
                    <a
                      href={node.repo_url}
                      rel="noreferrer noopener"
                      target="_blank"
                      className="inline-flex max-w-full items-center gap-1.5 text-signal hover:underline"
                    >
                      <span className="truncate">{node.repo_url}</span>
                      <ExternalLink aria-hidden="true" className="size-3.5 shrink-0" />
                    </a>
                  ) : (
                    t.node.none
                  )}
                </dd>
                <dt className="text-muted">{t.repo.branch}</dt>
                <dd>{node.default_branch ? <code className="text-ink">{node.default_branch}</code> : t.node.none}</dd>
              </dl>
            </Panel>
          )}
        </div>
      )}
      {project && node.repository && <RepositoryPanel node={node} repository={node.repository} locale={locale} labels={t} />}
      {!project && childrenList}
    </div>
  );

  return (
    <>
      <Crumbs
        items={[
          rootCrumb,
          ...(node?.path ?? []).map((p) => ({ label: p.name, href: catalogHref(locale, p.id), node: p.id })),
          ...(node ? [{ label: node.name, node: node.id }] : []),
        ]}
      />

      {node ? (
        <NodeHero
          kind={node.kind}
          icon={<KindIcon kind={node.kind} tile className="size-14 rounded-2xl shadow-elev-1 [&>svg]:size-7" />}
          title={node.name}
          actions={actions}
          meta={
            <>
              <Badge tone="outline" className="bg-surface/70">
                {t.kinds[node.kind]}
              </Badge>
              <code className="inline-flex h-6 items-center rounded-md bg-surface px-2 font-mono text-xs text-ink ring-1 ring-line ring-inset">
                {node.slug}
              </code>
              {!readable && <Badge tone="outline">{t.navigateOnly}</Badge>}
              {readable && activity?.processes && <ProcessBadges processes={activity.processes} labels={t.activity} locale={locale} />}
              {readable && activity?.summary && <SummaryBadges summary={activity.summary} labels={t.activity} />}
              {project && readable && <BranchPicker projectId={node.id} current={branch} labels={t.branches} onSelect={selectBranch} />}
              {managed && (
                <Badge tone="outline" className="bg-surface/70">
                  <GitBranch aria-hidden="true" className="size-3" />
                  {t.repoMeta.managed}
                </Badge>
              )}
            </>
          }
        >
          {project && readable && heroLinks.length > 0 && (
            <LinkIconRow links={heroLinks} locale={locale} labels={t.linkIcons} size={20} className={labelEntries.length > 0 ? "mb-3" : undefined} />
          )}
          {labelEntries.length > 0 && (
            <ul aria-label={t.node.labels} className="flex flex-wrap gap-1.5">
              {labelEntries.map(([k, v]) => (
                <li key={k}>
                  <LabelChip name={k} value={v} />
                </li>
              ))}
            </ul>
          )}
        </NodeHero>
      ) : (
        <PageHeader title={t.title} lead={t.lead} actions={addButton} glyph={FolderTree} />
      )}

      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}

      {!node ? (
        childrenList
      ) : tabs.length > 1 ? (
        <Tabs value={tab} onValueChange={selectTab} activationMode="manual">
          <TabsList aria-label={t.tabs.label}>
            {tabs.map((name) => {
              const Icon = tabIcons[name];
              return (
                <TabsTrigger key={name} value={name}>
                  <Icon aria-hidden="true" />
                  {t.tabs[name]}
                </TabsTrigger>
              );
            })}
          </TabsList>
          <TabsContent value="overview">{overview}</TabsContent>
          {project && (
            <>
              <TabsContent value="about">
                <DescriptionTab node={node} locale={locale} labels={t} canWrite={canWrite} />
              </TabsContent>
              <TabsContent value="deployments">
                <DeploymentsTab
                  key={branch ?? ""}
                  projectId={node.id}
                  branch={branch}
                  locale={locale}
                  labels={t}
                  onConnect={() => selectTab("connect")}
                />
              </TabsContent>
              <TabsContent value="events">
                <EventsTab projectId={node.id} locale={locale} labels={t} onConnect={() => selectTab("connect")} />
              </TabsContent>
              <TabsContent value="docs">
                <DocsTab
                  node={node}
                  branch={branchParam}
                  doc={doc}
                  locale={locale}
                  labels={t}
                  canWrite={canWrite}
                  reload={docsReload}
                  onCollect={refreshActivity}
                  onOpen={(nextBranch, nextDoc) =>
                    router.replace(catalogHref(locale, node.id, "docs", nextBranch, nextDoc), { scroll: false })
                  }
                />
              </TabsContent>
            </>
          )}
          {tabs.includes("settings") && (
            <TabsContent value="settings">
              <NodeSettings node={node} sections={sections} section={section} branch={branchParam} locale={locale} labels={t}>
                {section === "general" && (
                  <SettingsGeneral
                    node={node}
                    labels={t}
                    fail={fail}
                    canWrite={canWrite}
                    canMove={canMove}
                    canDelete={canDelete}
                    superadmin={superadmin}
                    onSaved={changed}
                    onDeleted={() => {
                      onChanged();
                      router.push(catalogHref(locale, node.parent_id));
                    }}
                  />
                )}
                {section === "branches" && <BranchesTab projectId={node.id} locale={locale} labels={t} canWrite={canWrite} />}
                {section === "links" && <LinksTab node={node} branch={branch} locale={locale} labels={t} canWrite={canWrite} />}
                {section === "docs" && <DocsSettingsTab node={node} locale={locale} labels={t} canWrite={canWrite} onChanged={refreshActivity} />}
                {section === "keys" && <ProjectKeys projectId={node.id} locale={locale} labels={t} />}
                {section === "access" && <AccessTab nodeId={node.id} labels={t} />}
                {section === "connect" && (
                  <IngestHowTo
                    projectId={node.id}
                    labels={t.howTo}
                    onKeys={sections.includes("keys") ? () => router.replace(catalogHref(locale, node.id, "keys"), { scroll: false }) : undefined}
                  />
                )}
                {section === "forge" && (
                  <ForgeTab nodeId={node.id} locale={locale} labels={t} canWrite={canWrite} canAccess={can("catalog.access")} onSynced={changed} />
                )}
              </NodeSettings>
            </TabsContent>
          )}
        </Tabs>
      ) : (
        overview
      )}

      {canCreate && (
        <CreateDialog
          open={dialog === "create"}
          onOpenChange={(open) => setDialog(open ? "create" : null)}
          parent={node}
          kinds={kinds}
          labels={t}
          fail={fail}
          onCreated={changed}
        />
      )}
    </>
  );
}
