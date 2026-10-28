"use client";

import {
  Cable,
  FolderInput,
  Network,
  Plug,
  RefreshCw,
  ShieldCheck,
  SlidersHorizontal,
  Trash2,
} from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { apiSend, errorCode } from "@/lib/api";
import { serverUrl, type McpServer } from "@/lib/mcp";
import { checkServer, loadServer } from "@/lib/mcpApi";
import { placeTrail } from "@/lib/placeCrumbs";
import { can } from "@/lib/rights";

import { Markdown } from "../catalog/Markdown";
import { DangerZone } from "../places/DangerZone";
import { PlaceMoveDialog } from "../places/PlaceMoveDialog";
import { useSession } from "../SessionProvider";
import { Crumbs, type Crumb } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { NodeHero } from "../ui/NodeHero";
import { Panel } from "../ui/Panel";
import { SectionMenu } from "../ui/SectionMenu";
import { SkeletonPanel } from "../ui/Skeleton";
import { Tabs, TabsList, TabsTrigger } from "../ui/Tabs";
import { AccessTab } from "./AccessTab";
import { CallsTab } from "./CallsTab";
import { ConnectTab } from "./ConnectTab";
import { ProxyForm } from "./ProxyForm";
import { ServerIcon, StateBadge } from "./ServerCard";
import { ServerForm } from "./ServerForm";
import { ToolsTab } from "./ToolsTab";

type Tab = "connect" | "overview" | "tools" | "calls" | "settings";

type Section = "general" | "connection" | "proxy" | "access";

const sectionIcons = {
  general: SlidersHorizontal,
  connection: Cable,
  proxy: Network,
  access: ShieldCheck,
} as const;

function Overview({
  server,
  locale,
  labels: t,
  onChecked,
}: {
  server: McpServer;
  locale: Locale;
  labels: Messages["mcp"];
  onChecked: (s: McpServer) => void;
}) {
  const p = t.page;
  const { errors } = useUiText();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const check = async () => {
    setBusy(true);
    setError(null);
    try {
      onChecked(await checkServer(server.id));
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    } finally {
      setBusy(false);
    }
  };
  const last = server.last_check;
  return (
    <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
      <Panel title={p.about}>
        {server.description.trim() ? (
          <Markdown source={server.description} />
        ) : (
          <p className="text-sm text-muted">{p.noDescription}</p>
        )}
        {server.server_info?.instructions && (
          <div className="mt-4 grid gap-1">
            <h3 className="text-sm font-medium text-muted">{p.instructions}</h3>
            <p className="text-sm whitespace-pre-line text-ink-2">
              {server.server_info.instructions}
            </p>
          </div>
        )}
      </Panel>
      <div className="grid gap-4">
        {server.tags.length > 0 && (
          <Panel title={p.tags}>
            <ul className="flex flex-wrap gap-1.5">
              {server.tags.map((tag) => (
                <li key={tag}>
                  <Badge>{tag}</Badge>
                </li>
              ))}
            </ul>
          </Panel>
        )}
        <Panel
          title={p.check}
          actions={
            server.can_write && (
              <Button size="sm" busy={busy} onClick={check}>
                <RefreshCw aria-hidden="true" className="size-4" /> {p.checkNow}
              </Button>
            )
          }
        >
          <dl className="grid gap-2 text-sm">
            <div className="flex justify-between gap-3">
              <dt className="text-muted">{t.state}</dt>
              <dd>
                <StateBadge server={server} labels={t} />
              </dd>
            </div>
            <div className="flex justify-between gap-3">
              <dt className="text-muted">{p.checked}</dt>
              <dd title={last ? when(last.at, locale, "") : undefined}>
                {last ? ago(last.at, locale, "") : p.never}
              </dd>
            </div>
            {last?.code && (
              <div className="grid gap-1">
                <dt className="text-muted">{p.error}</dt>
                <dd className="text-danger">{errorText(errors, last.code)}</dd>
                {last.detail && (
                  <dd className="font-mono text-xs break-all text-muted">
                    {last.detail}
                  </dd>
                )}
              </div>
            )}
            {last?.truncated && <dd className="text-amber">{p.truncated}</dd>}
            {server.server_info && (
              <div className="flex justify-between gap-3">
                <dt className="text-muted">{p.serverInfo}</dt>
                <dd className="text-right">
                  {server.server_info.title || server.server_info.name}{" "}
                  {server.server_info.version}
                </dd>
              </div>
            )}
            <div className="flex justify-between gap-3">
              <dt className="text-muted">{p.tools}</dt>
              <dd>{server.tools_count}</dd>
            </div>
          </dl>
          <Message
            note={error ? { kind: "error", text: error } : null}
            className="mt-3"
          />
        </Panel>
      </div>
    </div>
  );
}

export function ServerPage({
  locale,
  labels: t,
}: {
  locale: Locale;
  labels: Messages["mcp"];
}) {
  const params = useSearchParams();
  const router = useRouter();
  const { errors, common } = useUiText();
  const id = params.get("id") ?? "";
  const tab = (params.get("tab") as Tab | null) ?? "overview";
  const [server, setServer] = useState<McpServer | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [moving, setMoving] = useState(false);
  const { session } = useSession();
  const user = session.status === "signed-in" ? session.user : null;

  const load = useCallback(async () => {
    try {
      setServer(await loadServer(id));
      setError(null);
    } catch (e) {
      setError(
        errorCode(e) === "not_found"
          ? t.page.notFound
          : errorText(errors, errorCode(e)),
      );
    }
  }, [id, errors, t.page.notFound]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const go = (next: Tab) => {
    const q = new URLSearchParams({ id });
    if (next !== "overview") q.set("tab", next);
    router.replace(`/${locale}/mcp/server?${q.toString()}`, { scroll: false });
  };

  const remove = async (): Promise<string | null> => {
    if (!server) return null;
    try {
      await apiSend("DELETE", serverUrl(server.id));
      router.push(
        server.folder_id
          ? `/${locale}/mcp?folder=${server.folder_id}`
          : `/${locale}/mcp?space=${server.organization_id ?? "global"}`,
      );
      return null;
    } catch (e) {
      return errorText(errors, errorCode(e));
    }
  };

  if (error && !server) return <EmptyState icon={Plug} title={error} />;
  if (!server) return <SkeletonPanel lines={6} label={t.title} />;
  const crumbs: Crumb[] = [
    ...placeTrail({
      section: "mcp",
      locale,
      title: t.title,
      global: t.scopeGlobal,
      space: server.organization_id ?? "global",
      spaceName: server.organization?.name ?? t.scopeGlobal,
      folders: server.ancestors,
      current: "item",
    }),
    { label: server.title },
  ];
  const insider = server.can_write || server.can_manage;
  const sections: Section[] = server.can_write
    ? ["general", "connection", "proxy", "access"]
    : ["access"];
  const asked = params.get("section") as Section | null;
  const section: Section =
    asked && sections.includes(asked) ? asked : (sections[0] ?? "access");
  const menu = sections.map((key) => ({
    key,
    href: `/${locale}/mcp/server?id=${server.id}&tab=settings&section=${key}`,
    label: t.page.sections[key],
    icon: sectionIcons[key],
    current: key === section,
  }));
  return (
    <>
      <Crumbs items={crumbs} />
      <NodeHero
        kind="project"
        icon={<ServerIcon server={server} size="lg" />}
        title={server.title}
        meta={
          <>
            <code className="inline-flex h-6 items-center rounded-md bg-surface px-2 font-mono text-xs text-ink ring-1 ring-line ring-inset">
              {server.slug}__*
            </code>
            <StateBadge server={server} labels={t} />
            {server.status !== "published" && (
              <Badge tone="amber">{t.statuses[server.status]}</Badge>
            )}
            {server.author && (
              <span className="text-muted">{server.author}</span>
            )}
            <span className="text-muted">
              {format(t.callsCount, { count: server.calls })}
            </span>
            <span
              className="text-muted"
              title={when(server.updated_at, locale, "")}
            >
              {format(t.updated, { when: ago(server.updated_at, locale, "") })}
            </span>
          </>
        }
      />
      <Message
        note={error ? { kind: "error", text: error } : null}
        className="mb-4"
      />
      <Tabs value={tab} onValueChange={(v) => go(v as Tab)}>
        <TabsList className="mb-4">
          <TabsTrigger
            value="connect"
            className="bg-signal-soft text-signal hover:bg-signal-soft hover:text-signal data-[state=active]:bg-primary data-[state=active]:text-primary-ink data-[state=active]:[&_svg]:text-primary-ink"
          >
            <Plug aria-hidden="true" />
            {t.page.connect}
          </TabsTrigger>
          <TabsTrigger value="overview">{t.page.overview}</TabsTrigger>
          <TabsTrigger value="tools">{t.page.toolsTab}</TabsTrigger>
          {insider && <TabsTrigger value="calls">{t.page.calls}</TabsTrigger>}
          {insider && (
            <TabsTrigger value="settings">{t.page.settings}</TabsTrigger>
          )}
        </TabsList>
      </Tabs>
      {tab === "connect" && (
        <ConnectTab server={server} locale={locale} labels={t} />
      )}
      {tab === "overview" && (
        <Overview
          server={server}
          locale={locale}
          labels={t}
          onChecked={setServer}
        />
      )}
      {tab === "tools" && <ToolsTab server={server} labels={t} />}
      {tab === "calls" && insider && (
        <CallsTab server={server} locale={locale} labels={t} />
      )}
      {tab === "settings" && insider && (
        <SectionMenu
          label={t.page.settings}
          groups={[{ key: "sections", items: menu }]}
        >
          {section === "general" && (
            <div className="grid gap-4">
              <Panel title={t.page.sections.general}>
                <ServerForm
                  part="general"
                  server={server}
                  locale={locale}
                  labels={t}
                  onSaved={setServer}
                />
              </Panel>
              <DangerZone
                title={t.page.danger}
                rows={[
                  {
                    key: "move",
                    text: common.places.moveText,
                    action: (
                      <Button onClick={() => setMoving(true)}>
                        <FolderInput aria-hidden="true" />
                        {common.places.move}
                      </Button>
                    ),
                  },
                  {
                    key: "delete",
                    text: t.page.deleteHint,
                    action: (
                      <Button
                        variant="danger"
                        onClick={() => setDeleting(true)}
                      >
                        <Trash2 aria-hidden="true" />
                        {t.page.delete}
                      </Button>
                    ),
                  },
                ]}
              />
              <PlaceMoveDialog
                open={moving}
                onOpenChange={setMoving}
                base="/v1/mcp"
                name={server.title}
                global={t.scopeGlobal}
                canGlobal={can(user, "mcp", "create")}
                current={{
                  folder: server.folder_id,
                  organization: server.organization_id,
                }}
                move={async (to) => {
                  setServer(
                    await apiSend<McpServer>("PATCH", serverUrl(server.id), {
                      folder_id: to.folder,
                      organization_id: to.organization,
                    }),
                  );
                }}
                onMoved={() => undefined}
              />
            </div>
          )}
          {section === "connection" && (
            <Panel title={t.page.sections.connection}>
              <ServerForm
                part="connection"
                server={server}
                locale={locale}
                labels={t}
                onSaved={setServer}
              />
            </Panel>
          )}
          {section === "proxy" && (
            <ProxyForm
              owner="servers"
              id={server.id}
              organization={server.organization_id}
              canEdit
              locale={locale}
              labels={t}
            />
          )}
          {section === "access" && <AccessTab server={server} labels={t} />}
        </SectionMenu>
      )}
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t.page.delete}
        text={format(t.page.deleteText, { name: server.title })}
        confirm={t.page.delete}
        onConfirm={remove}
      />
    </>
  );
}
