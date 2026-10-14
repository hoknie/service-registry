"use client";

import { BookOpen, ListOrdered, ListTree, PanelLeftClose, PanelLeftOpen } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { ensureSpace, knownSpace, spaceEntry, useDocRevalidate, useDocStore } from "@/lib/docSpaceStore";
import { DOC_NAVIGATE, docHref, parseDocLocation, splitPath, type DocLocation } from "@/lib/docPath";

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
import { PanelSection } from "./PanelSection";
import { DocSpaceTree } from "./DocSpaceTree";
import { ProjectPage } from "./ProjectPage";

export type DocLabels = { space: Messages["docSpace"]; catalog: CatalogLabels };

type Props = { locale: Locale; labels: DocLabels };

function usePanelOpen(): [boolean, (open: boolean) => void] {
  const [open, setOpen] = useState(() => {
    try {
      return window.localStorage.getItem("svcr-doc-panel") !== "collapsed";
    } catch {
      return true;
    }
  });
  return [
    open,
    (next: boolean) => {
      setOpen(next);
      try {
        window.localStorage.setItem("svcr-doc-panel", next ? "expanded" : "collapsed");
      } catch {
        return;
      }
    },
  ];
}

function currentLocation(): DocLocation {
  return parseDocLocation(window.location.pathname, window.location.search);
}

export function DocSpace({ locale, labels }: Props) {
  const { errors, common } = useUiText();
  const t = labels.space;
  const [loc, setLoc] = useState<DocLocation | null>(null);
  const [contents, setContents] = useState(false);
  const [headings, setHeadings] = useState<{ items: DocHeading[] } | null>(null);
  const [panelOpen, setPanelOpen] = usePanelOpen();

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
  const space = current?.space ?? null;
  const treeSpace = space ?? known?.space ?? null;
  const nodePath = space?.node ? [...space.ancestors.map((a) => a.slug), space.node.slug] : [];
  const treePath = treeSpace?.node ? [...treeSpace.ancestors.map((a) => a.slug), treeSpace.node.slug] : [];

  const crumbs = useCallback(() => {
    const items = [{ label: t.title, href: docHref([]) }];
    if (!space?.node) return items;
    const chain = [...space.ancestors, space.node];
    chain.forEach((n, i) => items.push({ label: n.name, href: docHref(chain.slice(0, i + 1).map((c) => c.slug)) }));
    return items;
  }, [space, t.title]);

  const headingItems = space?.node?.kind === "project" && headings ? headings.items : [];
  const sections = (
    <div className="grid min-w-0 gap-3">
      {headingItems.length >= 2 && (
        <PanelSection title={t.toc} icon={ListOrdered} storageKey="svcr-doc-toc">
          <DocToc headings={headingItems} label={t.toc} />
        </PanelSection>
      )}
      <PanelSection title={t.structure} icon={ListTree} storageKey="svcr-doc-tree">
        {loc && (
          <DocSpaceTree
            locale={locale}
            labels={labels}
            nodePath={treePath}
            branch={loc.branch}
            docPath={treeSpace?.doc_path ?? null}
            projectId={treeSpace?.node?.kind === "project" ? treeSpace.node.id : null}
          />
        )}
      </PanelSection>
    </div>
  );

  return (
    <>
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
          panelOpen ? "lg:grid-cols-[18rem_minmax(0,1fr)]" : "lg:grid-cols-[2.75rem_minmax(0,1fr)]",
        )}
      >
        <aside
          aria-label={t.contents}
          className="sticky top-20 hidden max-h-[calc(100dvh-6rem)] min-w-0 overflow-x-hidden overflow-y-auto rounded-card border border-line bg-surface p-2 shadow-elev-1 lg:block"
        >
          <div className={cn("flex items-center", panelOpen ? "mb-1 justify-end" : "justify-center")}>
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
          {panelOpen && sections}
        </aside>
        <section className="min-w-0">
          {!current && <SkeletonPanel lines={6} label={common.loading} />}
          {current?.error && <Message note={{ kind: "error", text: current.error }} />}
          {current?.missing && (
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
          {space && space.node?.kind === "project" && loc && (
            <ProjectPage
              key={space.node.id}
              locale={locale}
              labels={labels}
              space={space}
              branch={loc.branch}
              nodePath={nodePath}
              onHeadings={(items) => setHeadings({ items })}
            />
          )}
          {space && space.node?.kind !== "project" && <ContainerPage locale={locale} labels={labels} space={space} nodePath={nodePath} />}
        </section>
      </div>
      <Dialog open={contents} onOpenChange={setContents} title={t.contents}>
        {sections}
      </Dialog>
    </>
  );
}
