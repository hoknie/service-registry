"use client";

import { ChevronRight } from "lucide-react";
import { useEffect, useId, useState } from "react";

import type { Locale } from "@/i18n/config";
import type { DocSpace, DocSpaceNode } from "@/lib/api";
import { cn } from "@/lib/cn";
import { docHref, navigateDoc, startPage } from "@/lib/docPath";
import {
  ensureFiles,
  ensureSpace,
  filesEntry,
  isPublicSource,
  levelEntry,
  spaceEntry,
  useDocStore,
} from "@/lib/docSpaceStore";

import { KindIcon } from "../catalog/KindIcon";
import { useUiText } from "../UiText";
import { Select } from "../ui/Select";
import { SkeletonRows } from "../ui/Skeleton";
import { Spinner } from "../ui/Spinner";
import type { DocLabels } from "./DocSpace";
import { DocFileTree } from "./DocFileTree";
import { DocLink } from "./DocLink";

type Props = {
  locale: Locale;
  labels: DocLabels;
  nodePath: string[];
  branch: string | null;
  docPath: string | null;
  projectId: string | null;
};

const indent = (depth: number) => ({ paddingLeft: `${depth * 0.9}rem` });

export function DocSpaceTree({
  labels,
  nodePath,
  branch,
  docPath,
  projectId,
}: Props) {
  const { common } = useUiText();
  const t = labels.space;
  const selectId = useId();
  useDocStore();
  const [opened, setOpened] = useState<Set<string>>(new Set());
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const currentKey = nodePath.join("/");
  const spaceSlug = nodePath[0] ?? null;

  const spaceOf = (k: string): DocSpace | undefined => spaceEntry(k)?.value;
  const isLoading = (k: string) => !!spaceEntry(k)?.loading;
  const isFailed = (k: string) => spaceEntry(k)?.error !== undefined;

  useEffect(() => {
    const want = [
      "",
      ...nodePath
        .slice(0, -1)
        .map((_, i) => nodePath.slice(0, i + 1).join("/")),
      currentKey,
    ].filter((k, i, all) => all.indexOf(k) === i);
    for (const key of want) ensureSpace(key);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentKey]);

  const onPath = (key: string) =>
    currentKey === key || currentKey.startsWith(`${key}/`);
  const isOpenKey = (key: string) =>
    onPath(key) ? !collapsed.has(key) : opened.has(key);

  const project = projectId ? spaceOf(currentKey) : null;
  const docs = project?.docs ?? null;
  const fileBranch =
    branch ??
    docs?.default_key ??
    (docs
      ? (docs.branches.find((b) => b.name === docs.default_branch)?.name ??
        docs.branches[0]?.name ??
        null)
      : null);
  useEffect(() => {
    if (projectId && fileBranch) ensureFiles(projectId, fileBranch);
  }, [projectId, fileBranch]);
  const files =
    projectId && fileBranch
      ? filesEntry(projectId, fileBranch)?.value
      : undefined;

  const flip = (set: Set<string>, key: string, on: boolean) => {
    const next = new Set(set);
    if (on) next.add(key);
    else next.delete(key);
    return next;
  };

  const toggle = (key: string, project: boolean) => {
    const opening = !isOpenKey(key);
    if (onPath(key)) setCollapsed((prev) => flip(prev, key, !opening));
    else setOpened((prev) => flip(prev, key, opening));
    if (opening && !project) ensureSpace(key);
  };

  const render = (
    children: DocSpaceNode[],
    parent: string[],
    depth: number,
  ): React.ReactNode =>
    children.map((c) => {
      const path = [...parent, c.slug];
      const key = path.join("/");
      const isCurrent = key === currentKey;
      const isOpen = isOpenKey(key);
      const container = c.kind !== "project";
      const sub = spaceOf(key);
      const nodeBranch = c.kind === "project" && isCurrent ? fileBranch : null;
      const busy = container
        ? isLoading(key)
        : isOpen && !!levelEntry(c.id, nodeBranch, "")?.loading;
      return (
        <li key={c.id} className="min-w-0">
          <div
            className="flex min-w-0 items-center gap-0.5"
            style={indent(depth)}
          >
            <button
              type="button"
              onClick={() => toggle(key, !container)}
              aria-expanded={isOpen}
              aria-busy={busy || undefined}
              aria-label={c.name}
              className="motion-control grid size-6 shrink-0 place-items-center rounded text-muted hover:bg-surface-2 hover:text-ink"
            >
              {busy ? (
                <Spinner
                  className="size-3.5 text-signal"
                  label={common.loading}
                />
              ) : (
                <ChevronRight
                  aria-hidden="true"
                  className={cn(
                    "motion-control size-3.5",
                    isOpen && "rotate-90",
                  )}
                />
              )}
            </button>

            <DocLink
              href={docHref(
                path,
                c.kind === "project" && isCurrent ? branch : null,
              )}
              aria-current={isCurrent && !docPath ? "page" : undefined}
              title={c.name}
              className={cn(
                "motion-control flex min-w-0 flex-1 items-center gap-1.5 rounded-md px-1.5 py-1 text-sm",
                isCurrent
                  ? "bg-signal-soft font-medium text-signal"
                  : "text-ink-2 hover:bg-surface-2 hover:text-ink",
                c.kind === "project" &&
                  c.has_docs === false &&
                  !isCurrent &&
                  "text-muted",
              )}
            >
              <KindIcon kind={c.kind} className="size-3.5 shrink-0" />
              <span className="min-w-0 truncate">{c.name}</span>
            </DocLink>
          </div>
          {container && isOpen && sub && sub.children.length > 0 && (
            <ul className="grid min-w-0 animate-fade-in gap-0.5 pt-0.5">
              {render(sub.children, path, depth + 1)}
            </ul>
          )}
          {container && isOpen && sub && sub.children.length === 0 && (
            <p className="py-1 text-xs text-muted" style={indent(depth + 1)}>
              <span className="pl-7">{t.empty}</span>
            </p>
          )}
          {container && isOpen && isFailed(key) && (
            <button
              type="button"
              onClick={() => ensureSpace(key, true)}
              className="py-1 pl-7 text-xs text-danger hover:underline"
              style={indent(depth + 1)}
            >
              {t.retry}
            </button>
          )}
          {!container && isOpen && (
            <DocFileTree
              projectId={c.id}
              projectPath={path}
              branch={nodeBranch}
              linkBranch={isCurrent ? branch : null}
              openPath={
                isCurrent
                  ? (docPath ??
                    (files ? startPage(files.items.map((f) => f.path)) : null))
                  : null
              }
              depth={depth + 1}
              labels={labels.catalog.docs}
              emptyLabel={t.notCollected}
              retryLabel={t.retry}
            />
          )}
        </li>
      );
    });

  const root = spaceOf("");
  if (isPublicSource()) {
    if (!root) return <SkeletonRows rows={3} label={common.loading} />;
    const shown = projectId ? { id: projectId, path: nodePath } : root.node?.kind === "project" ? { id: root.node.id, path: [] } : null;
    if (shown) {
      return (
        <DocFileTree
          projectId={shown.id}
          projectPath={shown.path}
          branch={fileBranch}
          linkBranch={branch}
          openPath={docPath ?? (files ? startPage(files.items.map((f) => f.path)) : null)}
          depth={0}
          labels={labels.catalog.docs}
          emptyLabel={t.notCollected}
          retryLabel={t.retry}
        />
      );
    }
    return root.children.length === 0 ? (
      <p className="px-2 py-1 text-xs text-muted">{t.empty}</p>
    ) : (
      <ul className="grid min-w-0 gap-0.5">{render(root.children, [], 0)}</ul>
    );
  }
  const space = spaceSlug ? spaceOf(spaceSlug) : null;
  const spaceLoading = spaceSlug ? !space && !isFailed(spaceSlug) : false;

  return (
    <div className="grid min-w-0 gap-2">
      <div className="grid gap-1.5 px-1 pt-1">
        <label htmlFor={selectId} className="text-xs font-medium text-muted">
          {t.space}
        </label>
        {root ? (
          <Select
            id={selectId}
            value={spaceSlug ?? ""}
            placeholder={t.chooseSpace}
            onChange={(e) =>
              e.target.value && navigateDoc(docHref([e.target.value]))
            }
          >
            {root.children.map((c) => (
              <option key={c.id} value={c.slug}>
                {c.name}
              </option>
            ))}
          </Select>
        ) : (
          <SkeletonRows rows={1} label={common.loading} />
        )}
      </div>
      {spaceSlug && <hr className="border-line" />}
      {spaceLoading && (
        <p className="flex items-center gap-2 px-2 py-1 text-xs text-muted">
          <Spinner className="size-3.5 text-signal" />
          {common.loading}
        </p>
      )}
      {spaceSlug && isFailed(spaceSlug) && (
        <button
          type="button"
          onClick={() => ensureSpace(spaceSlug, true)}
          className="px-2 py-1 text-left text-xs text-danger hover:underline"
        >
          {t.retry}
        </button>
      )}
      {space && space.children.length === 0 && (
        <p className="px-2 py-1 text-xs text-muted">{t.empty}</p>
      )}
      {space && space.children.length > 0 && (
        <ul className="grid min-w-0 gap-0.5">
          {render(space.children, [spaceSlug!], 0)}
        </ul>
      )}
      {!spaceSlug && root && (
        <p className="px-2 py-1 text-xs text-muted">{t.chooseSpaceHint}</p>
      )}
    </div>
  );
}
