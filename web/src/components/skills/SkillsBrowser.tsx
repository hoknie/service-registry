"use client";

import { GitBranch, GitPullRequestArrow, ListChecks, Plug, Sparkles, Upload } from "lucide-react";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { apiGet } from "@/lib/api";
import { parentPath } from "@/lib/placeTree";
import { can } from "@/lib/rights";
import { filtersFrom, listQuery, placeHref, searching, SKILLS_BASE, type Skill, type SkillFilters, type SkillFolder, type SkillPage, type SkillSpace } from "@/lib/skills";
import { loadFolder, loadFolders } from "@/lib/skillsApi";

import { OrganizationSkills } from "../catalog/OrganizationSkills";
import { type AccessLabels } from "../catalog/AccessTab";
import { PlaceBrowser, type PlaceAdapter } from "../places/PlaceBrowser";
import { PlaceSearch } from "../places/PlaceSearch";
import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Checkbox } from "../ui/Field";
import { Select } from "../ui/Select";
import { DisabledBadge, FolderPower } from "./DisabledBadge";
import { FolderConnect } from "./FolderConnect";
import { ImportDialog } from "./ImportDialog";
import { SkillBadges, SkillCard } from "./SkillCard";
import { SkillSetSection } from "./SkillSetSection";
import { SourcesView } from "./SourcesView";

type Props = { locale: Locale; labels: Messages["skills"]; access: AccessLabels };

const empty: SkillFilters = { q: "", tags: [], status: "", active: false };

export function SkillsBrowser({ locale, labels: t, access }: Props) {
  const { session } = useSession();
  const { common } = useUiText();
  const user = session.status === "signed-in" ? session.user : null;
  const canGlobal = can(user, "skills", "create");
  const router = useRouter();
  const [importing, setImporting] = useState(false);

  const adapter = useMemo<PlaceAdapter<Skill, SkillFolder, SkillFilters>>(
    () => ({
      section: "skills",
      base: SKILLS_BASE,
      title: t.title,
      scopeGlobal: t.scopeGlobal,
      globalLead: t.globalLead,
      rootLead: t.spacesLead,
      folderNotFound: t.folderNotFound,
      rootIcon: Sparkles,
      folderLabels: { ...t.folder, deleteFolder: t.deleteFolder, deleteFolderText: t.deleteFolderText },
      parse: filtersFrom,
      href: placeHref,
      searching,
      empty,
      canGlobal,
      globalRight: "skills",
      access,
      list: (place, filters, cursor) => apiGet<SkillPage>(listQuery(place, filters, cursor)),
      loadFolder,
      loadFolders,
      loadSpace: async (org) => {
        const sp = await apiGet<SkillSpace>(`${SKILLS_BASE}/organizations/${org}`);
        return { description: sp.description, settings: sp.can_write, data: sp };
      },
      folderMeta: (f) => format(t.folderSkills, { count: f.skills }),
      folderTotal: (f) => f.skills,
      item: {
        kind: t.place.skill,
        id: (s) => s.id,
        name: (s) => s.title,
        slug: (s) => s.name,
        href: (loc, s) => `/${loc}/skills/skill?id=${s.id}`,
        icon: () => <Sparkles aria-hidden="true" className="size-4 shrink-0 text-signal" />,
        folderPath: (s) => parentPath(s.path),
        card: (s) => <SkillCard skill={s} locale={locale} labels={t} />,
      },
      cells: [
        { key: "tags", title: t.tags, className: "hidden md:table-cell", item: (s) => <SkillBadges skill={s} labels={t} max={3} />, folder: () => null },
        { key: "version", title: t.place.version, className: "font-mono text-xs", item: (s) => (s.version ? `v${s.version.version}` : t.noVersion), folder: (f) => <span className="font-sans text-muted">{format(t.folderSkills, { count: f.skills })}</span> },
        {
          key: "updated",
          title: common.places.updated,
          className: "hidden sm:table-cell text-muted",
          item: (s) => <span title={when(s.updated_at, locale, "")}>{ago(s.updated_at, locale, "")}</span>,
          folder: () => null,
        },
      ],
      search: ({ filters, go, root, writable }) => (
        <PlaceSearch
          q={filters.q}
          tags={filters.tags}
          onChange={(next) => go({ ...filters, ...next })}
          placeholder={root ? t.search : t.searchHere}
          tagsLabel={t.tags}
          suggest={async (draft, signal) =>
            (await apiGet<{ items: { tag: string; count: number }[] }>(`${SKILLS_BASE}/tags?q=${encodeURIComponent(draft)}`, signal)).items.map((i) => ({ value: i.tag, label: i.tag, detail: String(i.count) }))
          }
          extra={
            !root && (
              <>
                {writable && (
                  <Select className="w-40" aria-label={t.status} value={filters.status} onChange={(e) => go({ ...filters, status: e.target.value })}>
                    <option value="">{t.statusAny}</option>
                    <option value="published">{t.statusPublished}</option>
                    <option value="draft">{t.statusDraft}</option>
                    <option value="disabled">{t.statusDisabled}</option>
                  </Select>
                )}
                <Checkbox label={t.onlyActive} checked={filters.active === true} onChange={(e) => go({ ...filters, active: e.target.checked })} />
              </>
            )
          }
          reset={{ shown: searching(filters) || filters.status !== "" || filters.active === true, label: t.reset, onReset: () => go(empty) }}
        />
      ),
      addItem: ({ place, space }) => ({
        href: `/${locale}/skills/upload${place.folder ? `?folder=${place.folder}` : space && space !== "global" ? `?space=${space}` : ""}`,
        label: t.place.skill,
        icon: Upload,
      }),
      folderSections: (f) => [{ key: "sources", label: t.place.sources, icon: GitBranch, render: () => <SourcesView key={f.id} folderId={f.id} locale={locale} labels={t} /> }],
      spaceSections: (org) => [
        { key: "skills", label: t.place.skillsSection, icon: Sparkles, render: () => <OrganizationSkills key={org} node={{ id: org }} labels={t} /> },
        { key: "set", label: t.set.section, icon: ListChecks, render: () => <SkillSetSection key={org} org={org} labels={t} /> },
      ],
      addMore: ({ place, space, folder }) => [
        {
          key: "import",
          label: t.importDialog.menu,
          icon: GitPullRequestArrow,
          onSelect: () => setImporting(true),
          dialog: (
            <ImportDialog
              open={importing}
              onOpenChange={setImporting}
              parent={place.folder}
              space={space}
              secretsOn={folder ? folder.organization_id : space && space !== "global" ? space : null}
              locale={locale}
              labels={t}
              onImported={(f) => {
                setImporting(false);
                router.push(`${placeHref(locale, { folder: f.id, space: null })}&tab=settings&section=sources`);
              }}
            />
          ),
        },
      ],
      folderTabs: (f) => [{ key: "connect", label: t.page.connect, icon: Plug, accent: true, content: <FolderConnect key={f.id} folder={f} locale={locale} labels={t} /> }],
      folderGeneral: (f, changed) => <FolderPower folder={f} labels={t.off} onChanged={changed} />,
      folderBadge: (f) => <DisabledBadge item={f} labels={t.off} />,
    }),
    [t, locale, canGlobal, common, access, importing, router],
  );

  return <PlaceBrowser adapter={adapter} locale={locale} />;
}
