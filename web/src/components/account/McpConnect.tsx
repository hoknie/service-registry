"use client";

import { Bot } from "lucide-react";
import { useEffect, useState } from "react";

import { claudeCommand } from "@/lib/oauthApps";

import { Button } from "../ui/Button";
import { CodeBlock } from "../ui/CodeBlock";
import { Panel } from "../ui/Panel";
import type { TokenLabels } from "./TokenTable";

type Props = { labels: TokenLabels["mcp"]; onIssue: () => void };

function useOrigin(): string {
  const [origin, setOrigin] = useState("https://registry.example");
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setOrigin(window.location.origin);
  }, []);
  return origin;
}

function useOAuthEnabled(): boolean {
  const [enabled, setEnabled] = useState(false);
  useEffect(() => {
    const controller = new AbortController();
    fetch("/.well-known/oauth-protected-resource/api/mcp", { signal: controller.signal, credentials: "omit" })
      .then((r) => setEnabled(r.ok))
      .catch(() => setEnabled(false));
    return () => controller.abort();
  }, []);
  return enabled;
}

export function McpConnect({ labels: t, onIssue }: Props) {
  const origin = useOrigin();
  const oauth = useOAuthEnabled();
  const action = (
    <Button variant="primary" onClick={onIssue}>
      <Bot aria-hidden="true" />
      {t.issue}
    </Button>
  );
  return (
    <Panel title={t.title} description={t.lead} actions={action}>
      <div className="grid gap-4">
        <CodeBlock title={t.endpoint} code={`${origin}/api/mcp`} />
        {oauth && (
          <div className="grid gap-2">
            <CodeBlock title={t.oauth} code={claudeCommand(origin, null)} />
            <p className="text-sm text-muted">{t.oauthHint}</p>
          </div>
        )}
        <CodeBlock title={oauth ? t.withPat : t.claude} code={claudeCommand(origin, "<token>")} />
        <CodeBlock title={t.bridge} code={`SVCR_TOKEN=<token> svc-registry mcp:stdio --url ${origin} --token-env SVCR_TOKEN`} />
      </div>
    </Panel>
  );
}

export function McpCommand({ token, labels: t }: { token: string; labels: TokenLabels["mcp"] }) {
  const origin = useOrigin();
  return <CodeBlock className="mt-4" title={t.withToken} code={claudeCommand(origin, token)} />;
}
