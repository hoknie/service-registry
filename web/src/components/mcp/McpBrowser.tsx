"use client";

import { Eye, Network, Plug } from "lucide-react";
import Link from "next/link";
import { useMemo } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { apiGet } from "@/lib/api";
import { filtersFrom, listQuery, MCP_BASE, placeHref, searching, type McpFilters, type McpFolder, type McpServer, type McpServerPage, type McpSpace } from "@/lib/mcp";
import { loadFolder, loadFolders, loadSpace } from "@/lib/mcpApi";
import { can } from "@/lib/rights";

import { OrganizationMcp } from "../catalog/OrganizationMcp";
import { type AccessLabels } from "../catalog/AccessTab";
import { PlaceBrowser, type PlaceAdapter } from "../places/PlaceBrowser";
import { PlaceSearch } from "../places/PlaceSearch";
import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Select } from "../ui/Select";
import { ProxyForm } from "./ProxyForm";
import { ServerCard, ServerIcon, StateBadge } from "./ServerCard";

type Props = { locale: Locale; labels: Messages["mcp"]; access: AccessLabels };

const empty: McpFilters = { q: "", tags: [], state: "" };

export function McpBrowser({ locale, labels: t, access }: Props) {
  const { session } = useSession();
  const { common } = useUiText();
  const user = session.status === "signed-in" ? session.user : null;
  const canGlobal = can(user, "mcp", "create");
  const managesGlobal = can(user, "mcp", "update");

  const adapter = useMemo<PlaceAdapter<McpServer, McpFolder, McpFilters>>(
    () => ({
      section: "mcp",
      base: MCP_BASE,
      title: t.title,
      scopeGlobal: t.scopeGlobal,
      globalLead: t.globalLead,
      rootLead: t.spacesLead,
      folderNotFound: t.folderNotFound,
      rootIcon: Plug,
      folderLabels: { ...t.folder, deleteFolder: t.deleteFolder, deleteFolderText: t.deleteFolderText },
      parse: filtersFrom,
      href: placeHref,
      searching,
      empty,
      canGlobal,
      globalRight: "mcp",
      access,
      list: (place, filters, cursor) => apiGet<McpServerPage>(listQuery(place, filters, cursor)),
      loadFolder,
      loadFolders,
      loadSpace: async (org) => {
        const sp = await loadSpace(org);
        return { description: sp.description, settings: sp.can_write || sp.can_manage, data: sp };
      },
      folderMeta: (f) => format(t.folderServers, { count: f.servers }),
      folderTotal: (f) => f.servers,
      item: {
        kind: t.place.server,
        id: (s) => s.id,
        name: (s) => s.title,
        slug: (s) => `${s.slug}__*`,
        href: (loc, s) => `/${loc}/mcp/server?id=${s.id}`,
        icon: (s) => <ServerIcon server={s} size="xs" />,
        folderPath: (s) => s.ancestors.map((x) => x.name).join(" / "),
        card: (s) => <ServerCard server={s} locale={locale} labels={t} />,
      },
      cells: [
        { key: "tags", title: t.tags, className: "hidden md:table-cell", item: (s) => s.tags.slice(0, 3).join(", "), folder: () => null },
        { key: "state", title: t.place.state, item: (s) => <StateBadge server={s} labels={t} />, folder: (f) => <span className="text-muted">{format(t.folderServers, { count: f.servers })}</span> },
        { key: "tools", title: t.place.tools, numeric: true, className: "hidden sm:table-cell text-muted", item: (s) => s.tools_count, folder: () => null },
        {
          key: "updated",
          title: common.places.updated,
          className: "hidden sm:table-cell text-muted",
          item: (s) => <span title={when(s.updated_at, locale, "")}>{ago(s.updated_at, locale, "")}</span>,
          folder: () => null,
        },
      ],
      search: ({ filters, go, root }) => (
        <PlaceSearch
          q={filters.q}
          tags={filters.tags}
          onChange={(next) => go({ ...filters, ...next })}
          placeholder={root ? t.search : t.searchHere}
          tagsLabel={t.tags}
          extra={
            <Select className="w-44" aria-label={t.state} value={filters.state} onChange={(e) => go({ ...filters, state: e.target.value })}>
              <option value="">{t.stateAny}</option>
              <option value="ok">{t.states.ok}</option>
              <option value="failing">{t.states.failing}</option>
              <option value="unknown">{t.states.unknown}</option>
              <option value="disabled">{t.states.disabled}</option>
            </Select>
          }
          reset={{ shown: searching(filters), label: t.reset, onReset: () => go(empty) }}
        />
      ),
      addItem: ({ place, space }) => ({
        href: `/${locale}/mcp/new${place.folder ? `?folder=${place.folder}` : space ? `?space=${space}` : ""}`,
        label: t.place.server,
        icon: Plug,
      }),
      accessNote: ({ space }) => (
        <p>
          {t.place.accessNote}{" "}
          <Link className="text-signal hover:underline" href={`/${locale}/mcp/visibility${space && space !== "global" ? `?organization=${space}` : ""}`}>
            {t.visibility.title}
          </Link>
        </p>
      ),
      actions: ({ root, folder, space, spaceInfo }) => {
        const manages = space === "global" ? managesGlobal : Boolean((spaceInfo?.data as McpSpace | undefined)?.can_manage);
        if (root || folder || !manages) return null;
        return (
          <Button asChild>
            <Link href={`/${locale}/mcp/visibility${space && space !== "global" ? `?organization=${space}` : ""}`}>
              <Eye aria-hidden="true" className="size-4" /> {t.visibility.title}
            </Link>
          </Button>
        );
      },
      folderSections: (f) => [
        {
          key: "proxy",
          label: t.place.proxy,
          icon: Network,
          render: () => <ProxyForm key={f.id} owner="folders" id={f.id} organization={f.organization_id} canEdit={f.can_write} locale={locale} labels={t} />,
        },
      ],
      spaceSections: (org) => [{ key: "mcp", label: t.place.mcpSection, icon: Plug, render: () => <OrganizationMcp key={org} node={{ id: org }} locale={locale} labels={t} /> }],
    }),
    [t, locale, canGlobal, managesGlobal, common, access],
  );

  return <PlaceBrowser adapter={adapter} locale={locale} />;
}
