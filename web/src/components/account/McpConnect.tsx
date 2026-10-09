"use client";

import { Bot } from "lucide-react";
import { useEffect, useState } from "react";

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

export function claudeCommand(origin: string, token: string): string {
  return `claude mcp add --transport http registry ${origin}/api/mcp \\\n  --header "Authorization: Bearer ${token}"`;
}

export function McpConnect({ labels: t, onIssue }: Props) {
  const origin = useOrigin();
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
        <CodeBlock title={t.claude} code={claudeCommand(origin, "<token>")} />
        <CodeBlock title={t.bridge} code={`SVCR_TOKEN=<token> svc-registry mcp:stdio --url ${origin} --token-env SVCR_TOKEN`} />
      </div>
    </Panel>
  );
}

export function McpCommand({ token, labels: t }: { token: string; labels: TokenLabels["mcp"] }) {
  const origin = useOrigin();
  return <CodeBlock className="mt-4" title={t.withToken} code={claudeCommand(origin, token)} />;
}
