"use client";

import { BookOpen, ExternalLink, FileText } from "lucide-react";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { when } from "@/i18n/time";
import { apiGet, type CatalogNode, type DocSpace, type KnowledgeFiles } from "@/lib/api";
import { shortCommit } from "@/lib/commit";
import { docHref, fileName, isDirectory, navigateDoc, parentDir, parseDocLocation, splitPath, startPage } from "@/lib/docPath";
import { ensureFiles, filesEntry, filesUrl, isPublicSource, levelEntry, useDocStore } from "@/lib/docSpaceStore";
import { refQuery, splitVersion } from "@/lib/refs";

import { ApiCards } from "../apis/ApiCards";
import { KindIcon } from "../catalog/KindIcon";
import { DocViewer, type DocHeading } from "../knowledge/DocViewer";
import { LimitNotice } from "../knowledge/LimitNotice";
import { RefPicker } from "../knowledge/RefPicker";
import { useUiText } from "../UiText";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import type { DocLabels } from "./DocSpace";
import { DirectoryPage } from "./DirectoryPage";
import { DocLink } from "./DocLink";
import { WideToggle } from "./DocWide";

type Props = {
  locale: Locale;
  labels: DocLabels;
  space: DocSpace;
  branch: string | null;
  nodePath: string[];
  onHeadings: (headings: DocHeading[]) => void;
};

export function ProjectPage({ locale, labels, space, branch: branchParam, nodePath, onHeadings }: Props) {
  const { errors, common } = useUiText();
  const t = labels.space;
  const project = space.node!;
  const docs = space.docs;
  const branches = docs?.branches ?? [];
  const defaultBranch = docs?.default_key ?? branches.find((b) => b.name === docs?.default_branch)?.name ?? branches[0]?.name ?? null;
  const isPublic = isPublicSource();
  const branch = branchParam ?? defaultBranch;
  const [loadedNode, setNode] = useState<CatalogNode | null>(null);
  const node = isPublicSource() ? ({ id: project.id, repo_url: null, hosting: null } as unknown as CatalogNode) : loadedNode;

  useEffect(() => {
    if (isPublicSource()) return;
    let live = true;
    apiGet<CatalogNode>(`/v1/catalog/nodes/${project.id}`)
      .then((n) => live && setNode(n))
      .catch(() => live && setNode(null));
    return () => {
      live = false;
    };
  }, [project.id]);

  useDocStore();
  useEffect(() => {
    if (branch) ensureFiles(project.id, branch);
  }, [project.id, branch]);

  const entry = branch ? filesEntry(project.id, branch) : undefined;
  const current =
    branch && entry && (entry.value || !entry.loading)
      ? {
          branch,
          files: entry.value ?? null,
          missing: entry.error === "not_found",
          error: entry.error && entry.error !== "not_found" ? errorText(errors, entry.error) : null,
        }
      : null;
  const paths = current?.files?.items.map((f) => f.path) ?? [];
  const directory = isDirectory(paths, space.doc_path) ? space.doc_path : null;
  const open = directory ? null : (space.doc_path ?? (current?.files ? startPage(paths) : null));
  const linkBranch = branchParam;
  const hrefOf = (path: string | null) => docHref([...nodePath, ...(path ? splitPath(path) : [])], linkBranch);
  const canWrite = node?.permissions?.includes("catalog.write") ?? false;
  const catalogHref = `/${locale}/catalog?node=${project.id}&tab=docs${space.doc_path ? `&doc=${encodeURIComponent(space.doc_path)}` : ""}${
    branchParam ? `&${refQuery(branchParam).toString()}` : ""
  }`;
  const tags = docs?.tags ?? [];
  const selected = current ? splitVersion(current.branch) : null;

  const selectBranch = async (next: string) => {
    const nextParam = next === defaultBranch ? null : next;
    let keep = space.doc_path;
    if (keep) {
      try {
        const files = await apiGet<KnowledgeFiles>(filesUrl(project.id, next));
        const kept = keep;
        if (!files.items.some((f) => f.path === kept || f.path.startsWith(`${kept}/`))) keep = null;
      } catch {
        keep = null;
      }
    }
    navigateDoc(docHref([...nodePath, ...(keep ? splitPath(keep) : [])], nextParam));
  };

  const docOf = (href: string) => {
    const loc = parseDocLocation(new URL(href, window.location.origin).pathname, "");
    if (loc.segments.length <= nodePath.length) return null;
    if (nodePath.some((s, i) => s !== loc.segments[i])) return null;
    return loc.segments.slice(nodePath.length).join("/");
  };

  const notCollected = branches.length + (docs?.tags?.length ?? 0) === 0 || current?.missing;
  const noDocument = !!notCollected || !!current?.error || !!directory || (!!current?.files && (!open || !paths.includes(open)));
  const openTitle = open ? (levelEntry(project.id, branchParam, parentDir(open))?.value?.files.find((f) => f.path === open)?.title ?? null) : null;

  useEffect(() => {
    const label = directory ? fileName(directory) : open ? openTitle || fileName(open) : null;
    document.title = label ? `${label} — ${project.name}` : project.name;
  }, [directory, open, openTitle, project.name]);

  useEffect(() => {
    if (noDocument) onHeadings([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [noDocument]);

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-4">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-3">
          <KindIcon kind="project" tile className="size-11 rounded-xl [&>svg]:size-5" />
          <div className="min-w-0">
            <p className="text-xs font-medium tracking-wide text-muted uppercase">{t.kinds.project}</p>
            <h1 className="truncate text-xl font-semibold text-ink">{project.name}</h1>
            {current?.files && (
              <p className="mt-0.5 text-xs text-muted">
                {format(t.snapshot, { commit: shortCommit(current.files.snapshot.commit).short, when: when(current.files.snapshot.collected_at, locale, "") })}
              </p>
            )}
          </div>
        </div>
        <div className="flex flex-wrap items-end gap-3">
          <WideToggle />
          {branches.length + tags.length > 0 && branch && (
            <RefPicker branches={branches} tags={tags} value={branch} labels={labels.catalog.docs} onChange={(key) => void selectBranch(key)} />
          )}
          {!isPublic && <a href={catalogHref} className="inline-flex h-9 items-center gap-1.5 text-sm text-signal hover:underline">
            {t.openInCatalog}
            <ExternalLink aria-hidden="true" className="size-3.5" />
          </a>}
        </div>
      </header>

      {current?.files && !isPublic && (
        <LimitNotice snapshot={current.files.snapshot} canWrite={canWrite} settingsHref={`/${locale}/catalog?node=${project.id}&tab=settings&section=docs`} />
      )}
      {current?.error && <Message note={{ kind: "error", text: current.error }} />}
      {notCollected && (
        <EmptyState
          icon={BookOpen}
          title={t.notCollected}
          text={t.notCollectedText}
          action={
            canWrite && !isPublic ? (
              <a href={`/${locale}/catalog?node=${project.id}&tab=settings&section=docs`} className="text-signal hover:underline">
                {t.settings}
              </a>
            ) : undefined
          }
        />
      )}
      {!notCollected && !current && <SkeletonPanel lines={6} label={common.loading} />}
      {current?.files && open && !paths.includes(open) && (
        <EmptyState
          icon={FileText}
          title={
            selected?.kind === "tag" ? format(t.fileMissingTag, { tag: selected.name }) : format(t.fileMissing, { branch: current.branch })
          }
          action={
            <DocLink href={hrefOf(null)} className="text-signal hover:underline">
              {t.toStart}
            </DocLink>
          }
        />
      )}
      {current?.files && directory && (
        <DirectoryPage projectId={project.id} nodePath={nodePath} branch={branchParam} linkBranch={linkBranch} dir={directory} labels={t} />
      )}
      {current?.files && !open && !directory && (
        <Panel title={t.files}>
          <p className="mb-3 text-sm text-muted">{t.noMarkdown}</p>
          <ul className="grid gap-1 text-sm">
            {paths.map((p) => (
              <li key={p}>
                <DocLink href={hrefOf(p)} className="font-mono text-signal hover:underline">
                  {p}
                </DocLink>
              </li>
            ))}
          </ul>
        </Panel>
      )}
      {current?.files && !space.doc_path && !directory && !isPublic && (
        <ApiCards projectId={project.id} branch={current.branch} labels={labels.catalog.docs.api} hrefOf={(p) => hrefOf(p)} />
      )}
      {current?.files && node && open && paths.includes(open) && (
        <DocViewer
          node={node}
          branch={current.branch}
          path={open}
          paths={paths}
          docHref={hrefOf}
          docOf={docOf}
          onOpenDoc={(path) => navigateDoc(hrefOf(path))}
          onHeadings={onHeadings}
          labels={labels.catalog.docs}
        />
      )}
    </div>
  );
}
