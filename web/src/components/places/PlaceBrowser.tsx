"use client";

import { ChevronDown, FolderPlus, LayoutGrid, Plus, Settings, ShieldCheck, type LucideIcon } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Fragment, useCallback, useEffect, useMemo, useState, type ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { errorCode } from "@/lib/api";
import { useChildrenView } from "@/lib/childrenView";
import { placeTrail, type PlaceSection } from "@/lib/placeCrumbs";
import { countText, loadLevel, type ItemPage, type PlaceRow } from "@/lib/placeTree";

import { AccessTab, type AccessLabels } from "../catalog/AccessTab";
import { KindIcon } from "../catalog/KindIcon";
import { Markdown } from "../catalog/Markdown";
import { Crumbs } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Menu, MenuContent, MenuItem, MenuTrigger } from "../ui/Menu";
import { Message } from "../ui/Message";
import { NodeHero } from "../ui/NodeHero";
import { Panel } from "../ui/Panel";
import { SkeletonRows } from "../ui/Skeleton";
import { ChildrenSection } from "./ChildrenSection";
import { FolderGeneral, type FolderGeneralLabels } from "./FolderGeneral";
import { NewFolderDialog, type NewFolderLabels } from "./NewFolderDialog";
import { PlaceCard, PlaceCards } from "./PlaceCard";
import { PlaceShell } from "./PlaceShell";
import { PlaceTable, type PlaceColumn } from "./PlaceTable";
import { PlaceTreeGrid } from "./PlaceTreeGrid";
import { pickSection, SettingsSections, type SettingsSection } from "./SettingsSections";
import { SpaceIcon, useOrganizations, type SpaceRef } from "./spaces";

export type PlaceFolder = {
  id: string;
  parent_id: string | null;
  organization_id: string | null;
  organization: { id: string; slug: string; name: string } | null;
  slug: string;
  name: string;
  description: string;
  ancestors: { id: string; slug: string; name: string }[];
  folders: number;
  can_write: boolean;
};

export type Place = { folder: string | null; space: string | null };

export type PlaceFilters = { q: string; tags: string[] };

export type PlaceSpaceInfo = { description: string; settings: boolean; data: unknown };

export type PlaceContext<F, Q> = {
  locale: Locale;
  place: Place;
  space: string | null;
  folder: F | null;
  filters: Q;
  go: (next: Q) => void;
  root: boolean;
  writable: boolean;
  spaceInfo: PlaceSpaceInfo | null;
};

export type ExtraSection = SettingsSection & { render: () => ReactNode };

export type ItemCell<I, F> = { key: string; title: string; className?: string; numeric?: boolean; item: (i: I) => ReactNode; folder: (f: F) => ReactNode };

export type PlaceAdapter<I, F extends PlaceFolder, Q extends PlaceFilters> = {
  section: PlaceSection;
  base: "/v1/skills" | "/v1/mcp";
  title: string;
  scopeGlobal: string;
  globalLead: string;
  rootLead: string;
  folderNotFound: string;
  rootIcon: LucideIcon;
  folderLabels: FolderGeneralLabels & NewFolderLabels;
  parse: (search: string) => Q;
  href: (locale: Locale, place: Place, filters?: Q) => string;
  searching: (filters: Q) => boolean;
  empty: Q;
  canGlobal: boolean;
  globalRight: string;
  access: AccessLabels;
  accessNote?: (ctx: PlaceContext<F, Q>) => ReactNode;
  list: (place: Place, filters: Q, cursor: string | null) => Promise<ItemPage<I>>;
  loadFolder: (id: string) => Promise<F>;
  loadFolders: (parent: string | null) => Promise<{ items: F[] }>;
  loadSpace: (org: string) => Promise<PlaceSpaceInfo>;
  folderMeta: (f: F) => string;
  folderTotal: (f: F) => number;
  item: {
    kind: string;
    id: (i: I) => string;
    name: (i: I) => string;
    slug: (i: I) => string;
    href: (locale: Locale, i: I) => string;
    icon: (i: I) => ReactNode;
    folderPath: (i: I) => string;
    card: (i: I) => ReactNode;
  };
  cells: ItemCell<I, F>[];
  search: (ctx: PlaceContext<F, Q>) => ReactNode;
  addItem: (ctx: PlaceContext<F, Q>) => { href: string; label: string; icon: LucideIcon };
  actions?: (ctx: PlaceContext<F, Q>) => ReactNode;
  folderSections: (f: F, ctx: PlaceContext<F, Q>) => ExtraSection[];
  spaceSections: (org: string, ctx: PlaceContext<F, Q>) => ExtraSection[];
  addMore?: (ctx: PlaceContext<F, Q>, changed: () => void) => { key: string; label: string; icon: LucideIcon; onSelect: () => void; dialog: ReactNode }[];
  folderTabs?: (f: F) => { key: string; label: string; icon: LucideIcon; content: ReactNode; accent?: boolean }[];
  folderGeneral?: (f: F, changed: () => void) => ReactNode;
  folderBadge?: (f: F) => ReactNode;
};

type Entry<I, F> = { type: "folder"; folder: F } | { type: "item"; item: I } | { type: "space"; space: SpaceRef };

type Row<I, F> = PlaceRow<F, I> & { space?: SpaceRef };

type Loaded<I, F> = { key: string; folder: F | null; folders: F[]; page: ItemPage<I> };

export function PlaceBrowser<I, F extends PlaceFolder, Q extends PlaceFilters>({ adapter: a, locale }: { adapter: PlaceAdapter<I, F, Q>; locale: Locale }) {
  const router = useRouter();
  const params = useSearchParams();
  const { errors, common } = useUiText();
  const p = common.places;
  const search = params.toString();
  const place = useMemo<Place>(() => {
    const q = new URLSearchParams(search);
    return { folder: q.get("folder"), space: q.get("space") };
  }, [search]);
  const parse = a.parse;
  const filters = useMemo(() => parse(search), [parse, search]);
  const tabParam = params.get("tab");
  const sectionParam = params.get("section");
  const orgs = useOrganizations();
  const [view, setView] = useChildrenView();
  const [data, setData] = useState<Loaded<I, F> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [spaceInfo, setSpaceInfo] = useState<{ org: string; info: PlaceSpaceInfo } | null>(null);
  const [reload, setReload] = useState(0);

  const root = !place.folder && !place.space;
  const deep = a.searching(filters);
  const placeKey = place.folder ?? place.space ?? "root";
  const loadKey = `${placeKey}|${JSON.stringify(filters)}|${reload}`;
  const { list, loadFolder, loadFolders, folderNotFound } = a;

  const load = useCallback(async () => {
    try {
      if (root && !deep) {
        setData({ key: loadKey, folder: null, folders: [], page: { items: [], next_cursor: null } });
        setError(null);
        return;
      }
      const [f, sub, page] = await Promise.all([
        place.folder ? loadFolder(place.folder) : Promise.resolve(null),
        deep ? Promise.resolve({ items: [] as F[] }) : loadFolders(place.folder),
        list(place, filters, null),
      ]);
      const folders = place.folder ? sub.items : sub.items.filter((x) => (x.organization_id ?? "global") === place.space);
      setData({ key: loadKey, folder: f, folders, page });
      setError(null);
    } catch (e) {
      setError(errorCode(e) === "not_found" ? folderNotFound : errorText(errors, errorCode(e)));
      setData((prev) => prev ?? { key: loadKey, folder: null, folders: [], page: { items: [], next_cursor: null } });
    }
  }, [root, deep, loadKey, place, filters, loadFolder, loadFolders, list, errors, folderNotFound]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const folder = data?.key.startsWith(`${placeKey}|`) ? data.folder : null;
  const pending = !data || !data.key.startsWith(`${placeKey}|`);
  const space = place.folder ? (folder ? (folder.organization_id ?? "global") : null) : place.space;
  const orgSpace = !place.folder && place.space && place.space !== "global" ? place.space : null;
  const { loadSpace } = a;
  useEffect(() => {
    if (!orgSpace) return;
    let live = true;
    loadSpace(orgSpace)
      .then((info) => live && setSpaceInfo({ org: orgSpace, info }))
      .catch(() => live && setSpaceInfo(null));
    return () => {
      live = false;
    };
  }, [orgSpace, loadSpace, reload]);
  const info = orgSpace && spaceInfo?.org === orgSpace ? spaceInfo.info : null;

  const spaceName = space === "global" ? a.scopeGlobal : (orgs.find((o) => o.id === space)?.name ?? folder?.organization?.name ?? "");
  const writable = folder ? folder.can_write : space === "global" ? a.canGlobal : Boolean(space);
  const go = (next: Q) => router.replace(a.href(locale, place, next));
  const ctx: PlaceContext<F, Q> = { locale, place, space, folder, filters, go, root, writable, spaceInfo: info };
  const changed = () => setReload((n) => n + 1);

  const trail = placeTrail({
    section: a.section,
    locale,
    title: a.title,
    global: a.scopeGlobal,
    space,
    spaceName,
    folders: [...(folder?.ancestors ?? []), ...(folder ? [folder] : [])],
    current: folder ? "folder" : space ? "space" : "root",
  });

  if (error && !data?.folder && place.folder) {
    return (
      <>
        <Crumbs items={trail} />
        <EmptyState icon={a.rootIcon} title={error} className="mt-4" />
      </>
    );
  }
  if (!data) return <SkeletonRows rows={6} label={common.loading} />;

  const spaces: SpaceRef[] = [{ id: "global", slug: "global", name: a.scopeGlobal }, ...orgs];
  const shown = pending ? null : data;
  const items = shown?.page.items ?? [];
  const folders = deep ? [] : (shown?.folders ?? []);
  const entries: Entry<I, F>[] = root && !deep ? spaces.map((s) => ({ type: "space", space: s })) : [...folders.map((f) => ({ type: "folder" as const, folder: f })), ...items.map((i) => ({ type: "item" as const, item: i }))];
  const more = !!shown?.page.next_cursor;

  const loadMore = async () => {
    if (!shown?.page.next_cursor) return;
    const next = await list(place, filters, shown.page.next_cursor);
    setData({ ...shown, page: { items: [...shown.page.items, ...next.items], next_cursor: next.next_cursor } });
  };

  const folderHref = (id: string) => a.href(locale, { folder: id, space: null });
  const spaceHref = (id: string) => a.href(locale, { folder: null, space: id });
  const entryHref = (e: Entry<I, F>) => (e.type === "folder" ? folderHref(e.folder.id) : e.type === "space" ? spaceHref(e.space.id) : a.item.href(locale, e.item));
  const entryKey = (e: Entry<I, F>) => (e.type === "folder" ? e.folder.id : e.type === "space" ? `space:${e.space.id}` : a.item.id(e.item));

  const added = a.addItem(ctx);
  const extraAdds = a.addMore?.(ctx, changed) ?? [];
  const addButton = writable && !root && (
    <Menu>
      <MenuTrigger asChild>
        <Button variant="primary">
          <Plus aria-hidden="true" />
          {p.add}
          <ChevronDown aria-hidden="true" />
        </Button>
      </MenuTrigger>
      <MenuContent>
        <MenuItem asChild>
          <Link href={added.href}>
            <added.icon aria-hidden="true" />
            {added.label}
          </Link>
        </MenuItem>
        <MenuItem onSelect={() => setCreating(true)}>
          <FolderPlus aria-hidden="true" />
          {p.folder}
        </MenuItem>
        {extraAdds.map((m) => (
          <MenuItem key={m.key} onSelect={m.onSelect}>
            <m.icon aria-hidden="true" />
            {m.label}
          </MenuItem>
        ))}
      </MenuContent>
    </Menu>
  );

  const card = (e: Entry<I, F>) => {
    if (e.type === "item") return a.item.card(e.item);
    if (e.type === "space")
      return (
        <PlaceCard href={spaceHref(e.space.id)} icon={<SpaceIcon space={e.space.id} size="sm" />} name={e.space.name} kind={p.space}>
          {e.space.id === "global" && <p className="mt-3 text-sm text-muted">{a.globalLead}</p>}
        </PlaceCard>
      );
    return (
      <PlaceCard href={folderHref(e.folder.id)} icon={<KindIcon kind="folder" tile className="size-9" />} name={e.folder.name} kind={p.folder} slug={e.folder.slug}>
        <p className="mt-3 text-xs text-muted">{a.folderMeta(e.folder)}</p>
      </PlaceCard>
    );
  };

  const icon = (e: Entry<I, F>) => (e.type === "folder" ? <KindIcon kind="folder" /> : e.type === "space" ? <SpaceIcon space={e.space.id} size="xs" /> : a.item.icon(e.item));
  const entryName = (e: Entry<I, F>) => (e.type === "folder" ? e.folder.name : e.type === "space" ? e.space.name : a.item.name(e.item));
  const kindOf = (e: Entry<I, F>) => (e.type === "folder" ? p.folder : e.type === "space" ? p.space : a.item.kind);
  const slugOfEntry = (e: Entry<I, F>) => (e.type === "folder" ? e.folder.slug : e.type === "space" ? e.space.slug : a.item.slug(e.item));
  const columns: PlaceColumn<Entry<I, F>>[] = [
    { key: "kind", title: p.kind, className: "text-muted", cell: kindOf },
    { key: "slug", title: p.slug, cell: (e) => <code className="text-xs text-ink-2">{slugOfEntry(e)}</code> },
    ...a.cells.map((c) => ({
      key: c.key,
      title: c.title,
      className: c.className,
      numeric: c.numeric,
      cell: (e: Entry<I, F>) => (e.type === "item" ? c.item(e.item) : e.type === "folder" ? c.folder(e.folder) : null),
    })),
  ];
  const resultColumns: PlaceColumn<Entry<I, F>>[] = [
    { key: "folder", title: p.folder, className: "text-muted", cell: (e) => (e.type === "item" ? a.item.folderPath(e.item) || "—" : "") },
    ...columns.slice(1),
  ];
  const table = (cols: PlaceColumn<Entry<I, F>>[]) => (
    <PlaceTable rows={entries} rowKey={entryKey} nameTitle={p.name} href={entryHref} icon={icon} name={entryName} columns={cols} />
  );

  const level = (at: Place) => async (cursor: string | null) =>
    loadLevel<Row<I, F>>(
      cursor,
      async () => {
        const sub = await loadFolders(at.folder);
        const own = at.folder ? sub.items : sub.items.filter((x) => (x.organization_id ?? "global") === at.space);
        return own.map((f) => ({ id: f.id, parent_id: f.parent_id, children: (f.folders ?? 1) + a.folderTotal(f), folder: f }));
      },
      async (c) => {
        const page = await list(at, a.empty, c);
        return { items: page.items.map((i) => ({ id: a.item.id(i), parent_id: at.folder, children: 0, item: i })), next_cursor: page.next_cursor };
      },
    );
  const loadTree = async (parent: string, cursor: string | null) => {
    if (root && parent === "") return { items: spaces.map((s) => ({ id: `space:${s.id}`, parent_id: null, children: 1, space: s })), next: null };
    if (parent.startsWith("space:")) return level({ folder: null, space: parent.slice(6) })(cursor);
    return level(parent === "" ? place : { folder: parent, space: null })(cursor);
  };
  const rowEntry = (r: Row<I, F>): Entry<I, F> => (r.space ? { type: "space", space: r.space } : r.folder ? { type: "folder", folder: r.folder } : { type: "item", item: r.item as I });
  const tree = (
    <PlaceTreeGrid<Row<I, F>>
      root=""
      sourceKey={`${placeKey}|${reload}|${root ? orgs.map((o) => o.id).join(",") : ""}`}
      load={loadTree}
      labels={{ name: p.name, loading: p.loading, more: p.more, retry: p.retry, expand: p.expand, collapse: p.collapse, label_aria: p.treeLabel, emptyBranch: p.emptyBranch }}
      href={(r) => entryHref(rowEntry(r))}
      rowName={(r) => entryName(rowEntry(r))}
      columns={columns.map((c) => ({ key: c.key, title: c.title, className: c.className, numeric: c.numeric, cell: (r: Row<I, F>) => c.cell(rowEntry(r)) }))}
      name={(r) => {
        const e = rowEntry(r);
        return (
          <>
            {icon(e)}
            <Link href={entryHref(e)} tabIndex={-1} className="min-w-0 truncate font-medium text-ink hover:underline">
              {entryName(e)}
            </Link>
          </>
        );
      }}
      empty={<p className="text-sm text-muted">{p.empty}</p>}
    />
  );

  const body = !shown ? (
    <SkeletonRows rows={4} label={a.title} />
  ) : entries.length === 0 ? (
    <EmptyState
      icon={a.rootIcon}
      title={deep ? p.noMatch : p.empty}
      action={!deep && addButton ? addButton : undefined}
    />
  ) : view === "tree" && !deep ? (
    tree
  ) : view === "tree" || view === "table" ? (
    table(deep ? resultColumns : columns)
  ) : (
    <PlaceCards>
      {entries.map((e) => (
        <li key={entryKey(e)} className="min-w-0">
          {card(e)}
        </li>
      ))}
    </PlaceCards>
  );

  const description = folder ? folder.description.trim() : (info?.description.trim() ?? "");
  const overview = (
    <div className="grid gap-6">
      {description && (
        <Panel title={p.about}>
          <Markdown source={description} />
        </Panel>
      )}
      <Message note={error ? { kind: "error", text: error } : null} />
      {a.search(ctx)}
      <ChildrenSection
        title={deep ? p.results : root ? p.spaces : p.contents}
        count={countText(entries.length, more)}
        view={view}
        onView={setView}
        labels={p}
        controls={entries.length > 0}
      >
        {body}
        {more && (
          <div className="flex justify-center">
            <Button onClick={loadMore}>{p.more}</Button>
          </div>
        )}
      </ChildrenSection>
    </div>
  );

  const orgOf = folder ? folder.organization_id : orgSpace;
  const accessSection: ExtraSection = {
    key: "access",
    label: p.access,
    icon: ShieldCheck,
    render: () => (
      <div className="grid gap-4">
        <Panel title={p.whoSees}>
          <div className="grid gap-2 text-sm text-ink-2">
            {folder && <p>{p.accessFolder}</p>}
            <p>{orgOf ? format(p.accessSpace, { name: spaceName }) : format(p.accessGlobal, { right: a.globalRight })}</p>
            {a.accessNote?.(ctx)}
          </div>
        </Panel>
        {orgOf && <AccessTab key={orgOf} nodeId={orgOf} labels={a.access} />}
      </div>
    ),
  };
  const sections: ExtraSection[] = folder
    ? folder.can_write
      ? [
          {
            key: "general",
            label: p.general,
            icon: Settings,
            render: () => (
              <div className="grid gap-6">
              {a.folderGeneral?.(folder, changed)}
              <FolderGeneral
                key={folder.id}
                base={a.base}
                folder={folder}
                labels={a.folderLabels}
                global={a.scopeGlobal}
                canGlobal={a.canGlobal}
                onSaved={changed}
                onDeleted={() => router.push(folder.parent_id ? folderHref(folder.parent_id) : spaceHref(space ?? "global"))}
              />
              </div>
            ),
          },
          ...a.folderSections(folder, ctx),
          accessSection,
        ]
      : []
    : orgSpace && info?.settings
      ? [...a.spaceSections(orgSpace, ctx), accessSection]
      : !root && space === "global" && a.canGlobal
        ? [accessSection]
        : [];
  const section = pickSection(
    sections.map((s) => s.key),
    sectionParam,
  );
  const selectTab = (next: string) => {
    const sp = new URLSearchParams(search);
    sp.delete("section");
    sp.delete("open");
    if (next === "overview") sp.delete("tab");
    else sp.set("tab", next);
    const q = sp.toString();
    router.replace(`/${locale}/${a.section}${q ? `?${q}` : ""}`, { scroll: false });
  };
  const sectionHref = (key: string) => {
    const sp = new URLSearchParams(search);
    sp.set("tab", "settings");
    sp.set("section", key);
    return `/${locale}/${a.section}?${sp.toString()}`;
  };
  const extraTabs = folder ? (a.folderTabs?.(folder) ?? []) : [];
  const tabs = [
    ...extraTabs,
    { key: "overview", label: p.overview, icon: LayoutGrid, content: overview },
    ...(sections.length > 0
      ? [
          {
            key: "settings",
            label: p.settings,
            icon: Settings,
            content: (
              <SettingsSections label={p.menu} sections={sections} current={section ?? ""} href={sectionHref}>
                {sections.find((s) => s.key === section)?.render()}
              </SettingsSections>
            ),
          },
        ]
      : []),
  ];

  const RootIcon = a.rootIcon;
  const hero = (
    <NodeHero
      kind={folder ? "folder" : space && space !== "global" ? "organization" : "project"}
      icon={
        root ? (
          <span aria-hidden="true" className="grid size-14 shrink-0 place-items-center rounded-2xl bg-gradient-to-br from-signal-soft to-cobalt-soft text-signal shadow-elev-1 ring-1 ring-line ring-inset">
            <RootIcon className="size-7" />
          </span>
        ) : folder ? (
          <KindIcon kind="folder" tile className="size-14 rounded-2xl shadow-elev-1 [&>svg]:size-7" />
        ) : (
          <SpaceIcon space={space ?? "global"} size="lg" />
        )
      }
      title={root ? a.title : (folder?.name ?? spaceName)}
      meta={
        <>
          {folder && (
            <code className="inline-flex h-6 items-center rounded-md bg-surface px-2 font-mono text-xs text-ink ring-1 ring-line ring-inset">{folder.slug}</code>
          )}
          {root && <span className="text-muted">{a.rootLead}</span>}
          {!root && !folder && space === "global" && <span className="text-muted">{a.globalLead}</span>}
          {folder && <span className="text-muted">{a.folderMeta(folder)}</span>}
          {folder && a.folderBadge?.(folder)}
        </>
      }
      actions={
        (addButton || a.actions) && (
          <>
            {addButton}
            {a.actions?.(ctx)}
          </>
        )
      }
    />
  );

  return (
    <>
      <Crumbs items={trail} />
      <PlaceShell id={placeKey} pending={pending} hero={hero} tabs={tabs} tab={tabParam === "settings" && sections.length > 0 ? "settings" : extraTabs.some((x) => x.key === tabParam) ? (tabParam ?? "overview") : "overview"} onTab={selectTab} tabsLabel={p.tabs} />
      <NewFolderDialog<F>
        open={creating}
        onOpenChange={setCreating}
        base={a.base}
        parent={place.folder}
        space={space}
        labels={a.folderLabels}
        onSaved={(f) => router.push(folderHref((f as PlaceFolder).id))}
      />
      {extraAdds.map((m) => (
        <Fragment key={m.key}>{m.dialog}</Fragment>
      ))}
    </>
  );
}

