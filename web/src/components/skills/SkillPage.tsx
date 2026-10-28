"use client";

import {
  ChevronRight,
  FileText,
  Folder,
  FolderOpen,
  GitBranch,
  ListTree,
  Plug,
  ChevronDown,
} from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode } from "@/lib/api";
import { cn } from "@/lib/cn";
import { placeTrail } from "@/lib/placeCrumbs";
import { can } from "@/lib/rights";
import {
  fileTree,
  skillUrl,
  withVersion,
  type Skill,
  type SkillFiles,
  type SkillVersion,
  type TreeNode,
  spaceOf,
} from "@/lib/skills";
import {
  loadFolder,
  loadSkill,
  loadVersions,
} from "@/lib/skillsApi";

import { PanelSection } from "../docs/PanelSection";
import { DocToc } from "../docs/DocToc";
import type { DocHeading } from "../knowledge/DocViewer";
import { useSession } from "../SessionProvider";
import { Crumbs, type Crumb } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import {
  Menu,
  MenuContent,
  MenuRadioGroup,
  MenuRadioItem,
  MenuTrigger,
} from "../ui/Menu";
import { Message } from "../ui/Message";
import { SkeletonPanel } from "../ui/Skeleton";
import { Tabs, TabsList, TabsTrigger } from "../ui/Tabs";
import { ConnectPanel } from "./ConnectPanel";
import { NodeHero } from "../ui/NodeHero";
import { DisabledBadge } from "./DisabledBadge";
import { SkillImage } from "./SkillCard";
import { SkillOverview } from "./SkillOverview";
import { SkillFileView } from "./SkillFileView";
import { SkillSettings } from "./SkillSettings";
import { SkillVersions } from "./SkillVersions";

type Props = { locale: Locale; labels: Messages["skills"] };

type Tab = "connect" | "overview" | "files" | "versions" | "settings";

function FileNode({
  node,
  open,
  depth,
  onOpen,
}: {
  node: TreeNode;
  open: string;
  depth: number;
  onOpen: (p: string) => void;
}) {
  const [expanded, setExpanded] = useState(() =>
    open.startsWith(`${node.path}/`),
  );
  const pad = { paddingLeft: `${depth * 0.9 + 0.5}rem` };
  if (node.file) {
    return (
      <li>
        <button
          type="button"
          style={pad}
          title={node.path}
          aria-current={node.path === open ? "page" : undefined}
          onClick={() => onOpen(node.path)}
          className={cn(
            "flex w-full items-center gap-1.5 rounded-md py-1 pr-2 text-left text-sm hover:bg-surface-2",
            node.path === open ? "bg-signal-soft text-signal" : "text-ink-2",
          )}
        >
          <FileText aria-hidden="true" className="size-3.5 shrink-0" />
          <span className="truncate">{node.file.title ?? node.name}</span>
        </button>
      </li>
    );
  }
  const Icon = expanded ? FolderOpen : Folder;
  return (
    <li>
      <button
        type="button"
        style={pad}
        aria-expanded={expanded}
        onClick={() => setExpanded(!expanded)}
        className="flex w-full items-center gap-1.5 rounded-md py-1 pr-2 text-left text-sm text-ink-2 hover:bg-surface-2"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 transition-transform",
            expanded && "rotate-90",
          )}
        />
        <Icon aria-hidden="true" className="size-3.5 shrink-0" />
        <span className="truncate">{node.name}</span>
      </button>
      {expanded && (
        <ul>
          {node.children.map((c) => (
            <FileNode
              key={c.path}
              node={c}
              open={open}
              depth={depth + 1}
              onOpen={onOpen}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

export function SkillPage({ locale, labels: t }: Props) {
  const params = useSearchParams();
  const router = useRouter();
  const { errors } = useUiText();
  const id = params.get("id") ?? "";
  const tab = (params.get("tab") as Tab | null) ?? (params.get("file") ? "files" : "overview");
  const versionParam = params.get("version");
  const fileParam = params.get("file") ?? "SKILL.md";
  const [skill, setSkill] = useState<Skill | null>(null);
  const [versions, setVersions] = useState<SkillVersion[]>([]);
  const [files, setFiles] = useState<SkillFiles | null>(null);
  const [crumbs, setCrumbs] = useState<Crumb[]>([
    { label: t.title, href: `/${locale}/skills` },
  ]);
  const [error, setError] = useState<string | null>(null);
  const [headings, setHeadings] = useState<DocHeading[]>([]);
  const [deleting, setDeleting] = useState(false);
  const { session } = useSession();
  const user = session.status === "signed-in" ? session.user : null;

  const href = useCallback(
    (next: { tab?: Tab; version?: string | null; file?: string | null }) => {
      const q = new URLSearchParams({ id });
      const v = next.version === undefined ? versionParam : next.version;
      const f = next.file === undefined ? params.get("file") : next.file;
      const tb = next.tab ?? tab;
      if (tb !== "overview") q.set("tab", tb);
      if (v) q.set("version", v);
      if (f && tb === "files") q.set("file", f);
      return `/${locale}/skills/skill?${q.toString()}`;
    },
    [id, versionParam, params, tab, locale],
  );

  const load = useCallback(async () => {
    try {
      const s = await loadSkill(id);
      setSkill(s);
      setVersions((await loadVersions(id)).items);
      const space = spaceOf(s.organization_id);
      const f = s.folder_id ? await loadFolder(s.folder_id) : null;
      const trail: Crumb[] = placeTrail({
        section: "skills",
        locale,
        title: t.title,
        global: t.scopeGlobal,
        space,
        spaceName: s.organization?.name ?? t.scopeGlobal,
        folders: f ? [...f.ancestors, { id: f.id, name: f.name }] : [],
        current: "item",
      });
      trail.push({ label: s.title });
      setCrumbs(trail);
    } catch (e) {
      setError(
        errorCode(e) === "not_found"
          ? t.notFound
          : errorText(errors, errorCode(e)),
      );
    }
  }, [id, locale, t, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  useEffect(() => {
    if (!skill) return;
    apiGet<SkillFiles>(withVersion(skillUrl(id, "/files"), versionParam))
      .then(setFiles)
      .catch((e: unknown) => setError(errorText(errors, errorCode(e))));
  }, [skill, id, versionParam, errors]);

  const tree = useMemo(() => fileTree(files?.items ?? []), [files]);
  const current =
    files?.items.find((f) => f.path === fileParam) ?? files?.items[0] ?? null;
  const go = (url: string) => router.replace(url, { scroll: false });

  const remove = async (): Promise<string | null> => {
    if (!skill) return null;
    try {
      await apiSend("DELETE", skillUrl(skill.id));
      router.push(
        skill.folder_id
          ? `/${locale}/skills?folder=${skill.folder_id}`
          : `/${locale}/skills`,
      );
      return null;
    } catch (e) {
      return errorText(errors, errorCode(e));
    }
  };

  if (error && !skill) return <EmptyState icon={FileText} title={error} />;
  if (!skill) return <SkeletonPanel lines={6} label={t.title} />;
  const latest = skill.version?.version ?? null;
  const shownVersion = files?.version ?? versionParam;
  return (
    <>
      <Crumbs items={crumbs} />
      <NodeHero
        kind="project"
        icon={<SkillImage skill={skill} size="hero" />}
        title={skill.title}
        meta={
          <>
            <code className="inline-flex h-6 items-center rounded-md bg-surface px-2 font-mono text-xs text-ink ring-1 ring-line ring-inset">
              {skill.name}
            </code>
            {skill.status === "draft" && <Badge tone="amber">{t.draft}</Badge>}
            <DisabledBadge item={skill} labels={t.off} />
            {skill.managed && (
              <Badge tone="outline">
                <GitBranch aria-hidden="true" className="size-3" /> {t.fromGit}
              </Badge>
            )}
          </>
        }
        actions={
          <Menu modal={false}>
            <MenuTrigger asChild>
              <Button aria-label={t.page.version} className="font-mono">
                {shownVersion ? `v${shownVersion}` : t.page.version}
                <ChevronDown aria-hidden="true" className="size-4 text-muted" />
              </Button>
            </MenuTrigger>
            <MenuContent className="max-h-80 overflow-y-auto">
              <MenuRadioGroup
                value={shownVersion ?? ""}
                onValueChange={(v) =>
                  go(href({ version: v === latest ? null : v }))
                }
              >
                {versions.map((v) => (
                  <MenuRadioItem key={v.version} value={v.version}>
                    <span className="font-mono whitespace-nowrap">
                      {v.version}
                    </span>
                    {v.yanked ? (
                      <Badge tone="danger">{t.page.yanked}</Badge>
                    ) : !v.available ? (
                      <Badge tone="amber">{t.page.unavailable}</Badge>
                    ) : v.prerelease ? (
                      <Badge tone="outline">{t.page.prerelease}</Badge>
                    ) : null}
                  </MenuRadioItem>
                ))}
              </MenuRadioGroup>
            </MenuContent>
          </Menu>
        }
      />
      <Message
        note={error ? { kind: "error", text: error } : null}
        className="mb-4"
      />
      <Tabs value={tab} onValueChange={(v) => go(href({ tab: v as Tab }))}>
        <TabsList className="mb-4">
          <TabsTrigger
            value="connect"
            className="bg-signal-soft text-signal hover:bg-signal-soft hover:text-signal data-[state=active]:bg-primary data-[state=active]:text-primary-ink data-[state=active]:[&_svg]:text-primary-ink"
          >
            <Plug aria-hidden="true" />
            {t.page.connect}
          </TabsTrigger>
          <TabsTrigger value="overview">{t.page.overview}</TabsTrigger>
          <TabsTrigger value="files">{t.page.view}</TabsTrigger>
          <TabsTrigger value="versions">{t.page.versions}</TabsTrigger>
          {skill.can_write && (
            <TabsTrigger value="settings">{t.page.settings}</TabsTrigger>
          )}
        </TabsList>
      </Tabs>
      {tab === "overview" && <SkillOverview skill={skill} locale={locale} labels={t} />}
      {tab === "files" && (
        <div className="grid gap-6 lg:grid-cols-[16rem_minmax(0,1fr)]">
          <aside
            aria-label={t.page.panel}
            className="grid content-start gap-4 lg:sticky lg:top-4 lg:max-h-[calc(100vh-2rem)] lg:overflow-y-auto"
          >
            <PanelSection
              title={t.page.files}
              icon={ListTree}
              storageKey="svcr-skill-files"
            >
              <ul>
                {tree.map((n) => (
                  <FileNode
                    key={`${shownVersion}:${n.path}`}
                    node={n}
                    open={current?.path ?? ""}
                    depth={0}
                    onOpen={(p) => go(href({ file: p }))}
                  />
                ))}
              </ul>
            </PanelSection>
            {headings.length > 0 && (
              <PanelSection
                title={t.page.toc}
                icon={FileText}
                storageKey="svcr-skill-toc"
              >
                <div data-toc-scroll className="max-h-80 overflow-y-auto">
                  <DocToc headings={headings} label={t.page.toc} />
                </div>
              </PanelSection>
            )}
          </aside>
          <div className="min-w-0">
            {shownVersion && latest && shownVersion !== latest && (
              <p className="mb-3 rounded-md bg-surface-2 px-3 py-2 text-sm text-ink-2">
                {format(t.page.notLatest, { version: shownVersion })}{" "}
                <button
                  type="button"
                  className="text-signal hover:underline"
                  onClick={() => go(href({ version: null }))}
                >
                  {t.page.toLatest}
                </button>
              </p>
            )}
            {files && current && shownVersion ? (
              <SkillFileView
                skillId={skill.id}
                version={shownVersion}
                file={current}
                files={files.items}
                onOpen={(p) => go(href({ file: p }))}
                onHeadings={setHeadings}
                labels={t}
              />
            ) : (
              <SkeletonPanel lines={6} label={t.title} />
            )}
          </div>
        </div>
      )}
      {tab === "versions" && (
        <SkillVersions
          skill={skill}
          versions={versions}
          locale={locale}
          labels={t}
          onOpen={(v) =>
            go(href({ tab: "files", version: v === latest ? null : v }))
          }
          onChanged={() => void load()}
        />
      )}
      {tab === "settings" && skill.can_write && (
        <SkillSettings
          key={skill.id}
          skill={skill}
          labels={t}
          onSaved={(s) => setSkill(s)}
          section={params.get("section")}
          sectionHref={(key) => `/${locale}/skills/skill?id=${skill.id}&tab=settings&section=${key}`}
          canGlobal={can(user, "skills", "create")}
          onDelete={() => setDeleting(true)}
          onReload={() => void load()}
        />
      )}
      {tab === "connect" && (
        <ConnectPanel
          skill={skill}
          versions={versions}
          locale={locale}
          labels={t}
        />
      )}
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t.page.delete}
        text={format(t.page.deleteText, { name: skill.name })}
        confirm={t.page.delete}
        onConfirm={remove}
      />
    </>
  );
}
