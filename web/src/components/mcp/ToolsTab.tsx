"use client";

import { Wrench } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { cn } from "@/lib/cn";
import type { McpServer, McpTool } from "@/lib/mcp";
import { disableTools, loadTools } from "@/lib/mcpApi";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { CodeBlock } from "../ui/CodeBlock";
import { EmptyState } from "../ui/EmptyState";
import { Checkbox } from "../ui/Field";
import { Message } from "../ui/Message";
import { SkeletonRows } from "../ui/Skeleton";

export function ToolsTab({
  server,
  labels: t,
}: {
  server: McpServer;
  labels: Messages["mcp"];
}) {
  const x = t.tools;
  const { errors } = useUiText();
  const [tools, setTools] = useState<McpTool[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const focused = useSearchParams().get("tool");
  const [open, setOpen] = useState<string | null>(focused);

  useEffect(() => {
    if (!focused || !tools) return;
    document
      .getElementById(`tool-${focused}`)
      ?.scrollIntoView({ block: "center" });
  }, [focused, tools]);

  useEffect(() => {
    loadTools(server.id)
      .then((r) => setTools(r.items))
      .catch((e: unknown) => setError(errorText(errors, errorCode(e))));
  }, [server.id, errors]);

  const toggle = async (tool: McpTool, enabled: boolean) => {
    if (!tools) return;
    const disabled = tools
      .filter((x) =>
        x.name === tool.name ? !enabled : !x.enabled && !x.skipped,
      )
      .map((x) => x.name);
    try {
      setTools((await disableTools(server.id, disabled)).items);
      setError(null);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  };

  if (tools === null)
    return error ? (
      <Message note={{ kind: "error", text: error }} />
    ) : (
      <SkeletonRows rows={4} label={x.title} />
    );
  if (tools.length === 0)
    return <EmptyState icon={Wrench} title={x.empty} text={x.emptyText} />;
  return (
    <div className="grid gap-3">
      <Message note={error ? { kind: "error", text: error } : null} />
      <ul className="grid gap-2">
        {tools.map((tool) => (
          <li
            key={tool.name}
            id={`tool-${tool.name}`}
            aria-current={tool.name === focused ? "true" : undefined}
            className={cn(
              "scroll-mt-24 rounded-card border bg-surface p-4 shadow-elev-1",
              tool.name === focused
                ? "border-signal ring-2 ring-signal/40"
                : "border-line",
            )}
          >
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 className="font-mono text-sm break-all text-ink">
                  {tool.gateway_name}
                </h3>
                {tool.title && (
                  <p className="text-sm text-ink-2">{tool.title}</p>
                )}
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {tool.annotations?.readOnlyHint && (
                  <Badge tone="cobalt">{x.readOnly}</Badge>
                )}
                {tool.annotations?.destructiveHint && (
                  <Badge tone="danger">{x.destructive}</Badge>
                )}
                {tool.skipped ? (
                  <Badge tone="amber">{x.skipped[tool.skipped]}</Badge>
                ) : server.can_write ? (
                  <Checkbox
                    label={x.enabled}
                    checked={tool.enabled}
                    onChange={(e) => toggle(tool, e.target.checked)}
                  />
                ) : (
                  !tool.enabled && <Badge tone="outline">{x.disabled}</Badge>
                )}
              </div>
            </div>
            {tool.description && (
              <p className="mt-2 text-sm whitespace-pre-line text-ink-2">
                {tool.description}
              </p>
            )}
            <button
              type="button"
              className="mt-2 text-xs text-signal hover:underline"
              aria-expanded={open === tool.name}
              onClick={() => setOpen(open === tool.name ? null : tool.name)}
            >
              {x.schema}
            </button>
            {open === tool.name && (
              <CodeBlock
                className="mt-2"
                code={JSON.stringify(tool.input_schema, null, 2)}
              />
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}
