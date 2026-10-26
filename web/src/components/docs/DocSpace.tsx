"use client";

import { BookOpen, ListOrdered, ListTree, PanelLeftClose, PanelLeftOpen, PanelRightClose, PanelRightOpen } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import type { DocSpace as DocSpaceData } from "@/lib/api";
import { ensureLevel, ensureSpace, knownSpace, levelEntry, nodePathOf, spaceEntry, useDocRevalidate, useDocStore } from "@/lib/docSpaceStore";
import { DOC_NAVIGATE, docHref, docPathCrumbs, parentDir, parseDocLocation, splitPath, type DocLocation } from "@/lib/docPath";

import type { CatalogLabels } from "../catalog/shared";
import { Crumbs } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { SkeletonPanel } from "../ui/Skeleton";
import { ContainerPage } from "./ContainerPage";
import { cn } from "@/lib/cn";

import type { DocHeading } from "../knowledge/DocViewer";
import { Tooltip } from "../ui/Tooltip";
import { DocLink } from "./DocLink";
import { DocToc } from "./DocToc";
import { DocWideProvider } from "./DocWide";
import { PanelSection } from "./PanelSection";
import { DocSpaceTree } from "./DocSpaceTree";
import { ProjectPage } from "./ProjectPage";

export type DocLabels = { space: Messages["docSpace"]; catalog: CatalogLabels };

type Props = { locale: Locale; labels: DocLabels };

function useStored(key: string, on: string, off: string, initial: boolean): [boolean, (value: boolean) => void] {
  const [value, setValue] = useState(() => {
    try {
      const raw = window.localStorage.getItem(key);
      return raw === null ? initial : raw === on;
    } catch {
      return initial;
    }
  });
  return [
    value,
    (next: boolean) => {
      setValue(next);
      try {
        window.localStorage.setItem(key, next ? on : off);
      } catch {
        return;
      }
    },
  ];
}

const COLUMNS: Record<string, string> = {
  "open-none": "lg:grid-cols-[18rem_minmax(0,1fr)]",
  "rail-none": "lg:grid-cols-[2.75rem_minmax(0,1fr)]",
  "open-open": "lg:grid-cols-[18rem_minmax(0,1fr)] xl:grid-cols-[18rem_minmax(0,1fr)_15rem]",
  "open-rail": "lg:grid-cols-[18rem_minmax(0,1fr)] xl:grid-cols-[18rem_minmax(0,1fr)_2.75rem]",
  "rail-open": "lg:grid-cols-[2.75rem_minmax(0,1fr)] xl:grid-cols-[2.75rem_minmax(0,1fr)_15rem]",
  "rail-rail": "lg:grid-cols-[2.75rem_minmax(0,1fr)] xl:grid-cols-[2.75rem_minmax(0,1fr)_2.75rem]",
};

type Shown = { space: DocSpaceData | null; missing: boolean; error: string | null };
type Page = { sig: string; loc: DocLocation; current: Shown; docKey: string };

const PREPARE_MS = 2000;

function currentLocation(): DocLocation {
  return parseDocLocation(window.location.pathname, window.location.search);
}

export function DocSpace({ locale, labels }: Props) {
  const { errors, common } = useUiText();
  const t = labels.space;
  const [loc, setLoc] = useState<DocLocation | null>(null);
  const [contents, setContents] = useState(false);
  const [headings, setHeadings] = useState<{ key: string; items: DocHeading[] } | null>(null);
  const [shown, setShown] = useState<Page | null>(null);
  const [forced, setForced] = useState<string | null>(null);
  const [panelOpen, setPanelOpen] = useStored("svcr-doc-panel", "expanded", "collapsed", true);
  const [tocOpen, setTocOpen] = useStored("svcr-doc-toc-panel", "expanded", "collapsed", true);
  const [wide, setWide] = useStored("svcr-doc-wide", "wide", "normal", false);

  useEffect(() => {
    document.documentElement.classList.toggle("doc-wide", wide);
    return () => document.documentElement.classList.remove("doc-wide");
  }, [wide]);

  useEffect(() => {
    const sync = () => {
      setLoc(currentLocation());
      setContents(false);
    };
    sync();
    window.addEventListener("popstate", sync);
    window.addEventListener(DOC_NAVIGATE, sync);
    return () => {
      window.removeEventListener("popstate", sync);
      window.removeEventListener(DOC_NAVIGATE, sync);
    };
  }, []);

  const key = loc ? loc.segments.join("/") : null;

  useDocStore();
  useDocRevalidate();
  useEffect(() => {
    if (key !== null && !knownSpace(splitPath(key))?.complete) ensureSpace(key);
  }, [key]);

  const entry = key !== null ? spaceEntry(key) : undefined;
  const known = loc ? knownSpace(loc.segments) : null;
  const current =
    known?.complete
      ? { space: known.space, missing: false, error: null }
      : entry && !entry.loading
        ? {
            space: entry.value ?? null,
            missing: entry.error === "not_found",
            error: entry.error && entry.error !== "not_found" ? errorText(errors, entry.error) : null,
          }
        : null;
  const latest: Page | null =
    current && loc
      ? {
          sig: [key, current.space?.node?.id ?? "", current.space?.doc_path ?? "", current.missing, current.error ?? ""].join("|"),
          loc,
          current,
          docKey: `${loc.segments.join("/")}|${loc.branch ?? ""}`,
        }
      : null;
  const latestProject = latest?.current.space?.node?.kind === "project" ? latest.current.space.node.id : null;
  const waits = !!latest && !!shown && !!latestProject && latestProject !== shown.current.space?.node?.id;
  const ready = !waits || headings?.key === latest?.docKey || forced === latest?.docKey;
  if (latest && ready && latest.sig !== shown?.sig) setShown(latest);
  const view = latest && ready ? latest : shown;
  const preparing = latest && !ready ? latest : null;
  const pending = (!latest && !!shown) || !!preparing;
  const viewLoc = view?.loc ?? loc;
  const docKey = view?.docKey ?? "";
  const space = view?.current.space ?? null;

  const preparingKey = preparing?.docKey ?? null;
  useEffect(() => {
    if (!preparingKey) return;
    const timer = setTimeout(() => setForced(preparingKey), PREPARE_MS);
    return () => clearTimeout(timer);
  }, [preparingKey]);
  const treeSpace = space ?? known?.space ?? null;
  const treePath = treeSpace ? nodePathOf(treeSpace) : [];

  const docPath = space?.doc_path ?? null;
  const projectId = space?.node?.kind === "project" ? space.node.id : null;
  const branchKey = viewLoc?.branch ?? null;
  useEffect(() => {
    if (projectId && docPath) ensureLevel(projectId, branchKey, parentDir(docPath));
  }, [projectId, branchKey, docPath]);
  const crumbs = useCallback(() => {
    const items = [{ label: space?.entry?.title ?? t.title, href: docHref([]) }];
    if (!space?.node) return items;
    const nodePath = nodePathOf(space);
    const chain = [...space.ancestors, space.node].slice(0, nodePath.length);
    chain.forEach((n, i) => items.push({ label: n.name, href: docHref(nodePath.slice(0, i + 1), i === chain.length - 1 ? branchKey : null) }));
    const parts = docPathCrumbs(docPath);
    parts.forEach((part, i) => {
      const last = i === parts.length - 1;
      const title = last && projectId ? levelEntry(projectId, branchKey, parentDir(part.path))?.value?.files.find((f) => f.path === part.path)?.title : null;
      items.push({ label: title || part.name, href: docHref([...nodePath, ...splitPath(part.path)], branchKey) });
    });
    return items;
  }, [space, t.title, docPath, projectId, branchKey]);

  const headingItems = space?.node?.kind === "project" && headings ? headings.items : [];
  const tocStale = pending || headings?.key !== docKey;
  const hasToc = headingItems.length >= 2;
  const toc = hasToc && <DocToc headings={headingItems} label={t.toc} />;
  const tree = viewLoc && (
    <div aria-busy={pending || undefined} className={cn("motion-control min-w-0", pending && "opacity-50 delay-150")}>
      <DocSpaceTree
        locale={locale}
        labels={labels}
        nodePath={treePath}
        branch={viewLoc.branch}
        docPath={treeSpace?.doc_path ?? null}
        projectId={treeSpace?.node?.kind === "project" ? treeSpace.node.id : null}
      />
    </div>
  );
  const tocSection = (className?: string) =>
    hasToc && (
      <PanelSection className={className} title={t.toc} icon={ListOrdered} storageKey="svcr-doc-toc" busy={tocStale}>
        {toc}
      </PanelSection>
    );
  const structureTitle = (
    <span className="flex min-w-0 flex-1 items-center gap-2 px-1.5 text-xs font-semibold tracking-wide text-muted uppercase">
      <ListTree aria-hidden="true" className="size-3.5 shrink-0" />
      <span className="truncate">{t.structure}</span>
    </span>
  );


  return (
    <DocWideProvider value={{ wide, setWide, wideLabel: t.wideView, normalLabel: t.normalView }}>
      <Crumbs items={crumbs()} />
      <div className="mb-4 flex items-center justify-between gap-3 lg:hidden">
        <h1 className="flex items-center gap-2 text-lg font-semibold text-ink">
          <BookOpen aria-hidden="true" className="size-5 text-signal" />
          {t.title}
        </h1>
        <Button size="sm" onClick={() => setContents(true)} aria-label={t.openContents}>
          <ListTree aria-hidden="true" />
          {t.contents}
        </Button>
      </div>
      <div
        className={cn(
          "grid items-start gap-6 transition-[grid-template-columns] duration-200 ease-out motion-reduce:transition-none",
          COLUMNS[`${panelOpen ? "open" : "rail"}-${hasToc ? (tocOpen ? "open" : "rail") : "none"}`],
        )}
      >
        <aside
          aria-label={t.contents}
          className="sticky top-20 hidden max-h-[calc(100dvh-6rem)] min-w-0 overflow-x-hidden overflow-y-auto rounded-card border border-line bg-surface p-2 shadow-elev-1 lg:block"
        >
          <div className={cn("flex items-center gap-2", panelOpen ? "mb-1" : "justify-center")}>
            {panelOpen && structureTitle}
            <Tooltip content={panelOpen ? t.panelHide : t.panelShow} side="right">
              <button
                type="button"
                onClick={() => setPanelOpen(!panelOpen)}
                aria-expanded={panelOpen}
                aria-label={panelOpen ? t.panelHide : t.panelShow}
                className="motion-control grid size-7 place-items-center rounded-md text-muted hover:bg-surface-2 hover:text-ink"
              >
                {panelOpen ? <PanelLeftClose aria-hidden="true" className="size-4" /> : <PanelLeftOpen aria-hidden="true" className="size-4" />}
              </button>
            </Tooltip>
          </div>
          {panelOpen && (
            <div className="grid min-w-0 gap-3">
              {tree}
              {tocSection("xl:hidden")}
            </div>
          )}
        </aside>
        <section aria-busy={pending || undefined} inert={pending} className={cn("motion-control min-w-0", pending && "opacity-60 delay-150")}>
          {!view && <SkeletonPanel lines={6} label={common.loading} />}
          {[view, preparing].map((page) => {
            if (!page) return null;
            const pageSpace = page.current.space;
            const path = pageSpace ? nodePathOf(pageSpace) : [];
            return (
              <div key={pageSpace?.node?.id ?? page.sig} hidden={page === preparing} className="min-w-0">
                {page.current.error && <Message note={{ kind: "error", text: page.current.error }} />}
                {page.current.missing && (
                  <EmptyState
                    icon={BookOpen}
                    title={t.notFound}
                    action={
                      <DocLink href={docHref([])} className="text-signal hover:underline">
                        {t.toRoot}
                      </DocLink>
                    }
                  />
                )}
                {pageSpace && pageSpace.node?.kind === "project" && (
                  <ProjectPage
                    locale={locale}
                    labels={labels}
                    space={pageSpace}
                    branch={page.loc.branch}
                    nodePath={path}
                    onHeadings={(items) => setHeadings({ key: page.docKey, items })}
                  />
                )}
                {pageSpace && pageSpace.node?.kind !== "project" && <ContainerPage locale={locale} labels={labels} space={pageSpace} nodePath={path} />}
              </div>
            );
          })}
        </section>
        {hasToc && (
          <aside
            aria-label={t.toc}
            className="sticky top-20 hidden max-h-[calc(100dvh-6rem)] min-w-0 overflow-x-hidden overflow-y-auto rounded-card border border-line bg-surface p-2 shadow-elev-1 xl:block"
          >
            <div className={cn("flex items-center gap-2", tocOpen ? "mb-1" : "justify-center")}>
              {tocOpen && (
                <span className="flex min-w-0 flex-1 items-center gap-2 px-1.5 text-xs font-semibold tracking-wide text-muted uppercase">
                  <ListOrdered aria-hidden="true" className="size-3.5 shrink-0" />
                  <span className="truncate">{t.toc}</span>
                </span>
              )}
              <Tooltip content={tocOpen ? t.tocHide : t.tocShow} side="left">
                <button
                  type="button"
                  onClick={() => setTocOpen(!tocOpen)}
                  aria-expanded={tocOpen}
                  aria-label={tocOpen ? t.tocHide : t.tocShow}
                  className="motion-control grid size-7 place-items-center rounded-md text-muted hover:bg-surface-2 hover:text-ink"
                >
                  {tocOpen ? <PanelRightClose aria-hidden="true" className="size-4" /> : <PanelRightOpen aria-hidden="true" className="size-4" />}
                </button>
              </Tooltip>
            </div>
            {tocOpen && <div className={cn("motion-control min-w-0", tocStale && "opacity-50 delay-150")}>{toc}</div>}
          </aside>
        )}
      </div>
      <Dialog open={contents} onOpenChange={setContents} title={t.contents}>
        <div className="grid min-w-0 gap-3">
          {tocSection()}
          <div className="grid min-w-0 gap-1.5">
            {structureTitle}
            {tree}
          </div>
        </div>
      </Dialog>
    </DocWideProvider>
  );
}
