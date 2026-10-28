"use client";

import { Plug } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { BUILTIN_ID, builtinServer, builtinTools, type McpBuiltin } from "@/lib/mcp";
import { loadBuiltin, putBuiltinDisabled } from "@/lib/mcpApi";

import { Crumbs } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Message } from "../ui/Message";
import { NodeHero } from "../ui/NodeHero";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import { Tabs, TabsList, TabsTrigger } from "../ui/Tabs";
import { ConnectTab } from "./ConnectTab";
import { ServerIcon, StateBadge } from "./ServerCard";
import { ToolsTab } from "./ToolsTab";

type Tab = "connect" | "overview" | "tools";

const ACCENT =
  "bg-signal-soft text-signal hover:bg-signal-soft hover:text-signal data-[state=active]:bg-primary data-[state=active]:text-primary-ink data-[state=active]:[&_svg]:text-primary-ink";

export function BuiltinServerPage({ locale, labels: t }: { locale: Locale; labels: Messages["mcp"] }) {
  const params = useSearchParams();
  const router = useRouter();
  const { errors } = useUiText();
  const raw = params.get("tab");
  const tab: Tab = raw === "connect" || raw === "tools" ? raw : "overview";
  const [builtin, setBuiltin] = useState<McpBuiltin | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadBuiltin()
      .then(setBuiltin)
      .catch((e: unknown) => setError(errorText(errors, errorCode(e))));
  }, [errors]);

  const go = (next: Tab) => {
    const q = new URLSearchParams({ id: BUILTIN_ID });
    if (next !== "overview") q.set("tab", next);
    router.replace(`/${locale}/mcp/server?${q.toString()}`, { scroll: false });
  };
  const source = useMemo(
    () => ({
      load: async () => ({ items: builtinTools(await loadBuiltin()) }),
      save: async (disabled: string[]) => {
        const next = await putBuiltinDisabled(disabled);
        setBuiltin(next);
        return { items: builtinTools(next) };
      },
    }),
    [],
  );
  const server = useCallback((b: McpBuiltin) => builtinServer(b, t.builtin.description), [t.builtin.description]);

  const crumbs = [
    { label: t.title, href: `/${locale}/mcp` },
    { label: t.scopeGlobal, href: `/${locale}/mcp?space=global` },
    { label: "Service Registry" },
  ];
  if (!builtin) {
    return (
      <>
        <Crumbs items={crumbs} />
        {error ? <Message note={{ kind: "error", text: error }} /> : <SkeletonPanel label={t.title} />}
      </>
    );
  }
  const s = server(builtin);
  return (
    <>
      <Crumbs items={crumbs} />
      <NodeHero
        kind="project"
        icon={<ServerIcon server={s} size="lg" />}
        title={s.title}
        meta={
          <>
            <code className="inline-flex h-6 items-center rounded-md bg-surface px-2 font-mono text-xs text-ink ring-1 ring-line ring-inset">{s.slug}</code>
            <StateBadge server={s} labels={t} />
            <Badge tone="cobalt">{t.builtin.badge}</Badge>
            <span className="text-muted">v{builtin.version}</span>
          </>
        }
      />
      <Tabs value={tab} onValueChange={(v) => go(v as Tab)}>
        <TabsList className="my-4">
          <TabsTrigger value="connect" className={ACCENT}>
            <Plug aria-hidden="true" />
            {t.page.connect}
          </TabsTrigger>
          <TabsTrigger value="overview">{t.page.overview}</TabsTrigger>
          <TabsTrigger value="tools">{t.page.toolsTab}</TabsTrigger>
        </TabsList>
      </Tabs>
      {tab === "connect" && <ConnectTab server={s} locale={locale} labels={t} />}
      {tab === "overview" && (
        <div className="grid gap-4">
          <Panel title={t.builtin.aboutTitle}>
            <p className="text-sm text-ink-2">{t.builtin.description}</p>
          </Panel>
          <Panel title={t.page.instructions}>
            <pre className="text-sm whitespace-pre-wrap text-ink-2">{builtin.instructions}</pre>
          </Panel>
        </div>
      )}
      {tab === "tools" && <ToolsTab server={s} labels={t} source={source} />}
    </>
  );
}
