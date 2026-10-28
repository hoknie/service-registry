"use client";

import Link from "next/link";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { agentConfig, mcpEndpoint, type McpServer } from "@/lib/mcp";

import { CodeBlock } from "../ui/CodeBlock";
import { Panel } from "../ui/Panel";

export function ConnectTab({
  server,
  locale,
  labels: t,
}: {
  server: McpServer;
  locale: Locale;
  labels: Messages["mcp"];
}) {
  const c = t.connect;
  const endpoint = mcpEndpoint(
    typeof window === "undefined" ? "" : window.location.origin,
  );
  return (
    <Panel title={c.title} description={c.lead}>
      <div className="grid gap-4">
        <CodeBlock title={c.endpoint} code={endpoint} />
        <CodeBlock title={c.claude} code={agentConfig(endpoint, "claude")} />
        <CodeBlock title={c.json} code={agentConfig(endpoint, "json")} />
        {server.builtin ? (
          <p className="text-sm text-ink-2">{t.builtin.noPrefix}</p>
        ) : (
          <p className="text-sm text-ink-2">
            {format(c.prefix, { prefix: `${server.slug}__` })}
          </p>
        )}
        <p className="text-sm text-ink-2">
          {c.pin}{" "}
          <Link
            className="text-signal hover:underline"
            href={`/${locale}/account?tab=tokens`}
          >
            {c.tokens}
          </Link>
        </p>
      </div>
    </Panel>
  );
}
