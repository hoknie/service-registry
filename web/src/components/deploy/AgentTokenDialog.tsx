"use client";

import { KeyDialog } from "../catalog/KeyDialog";
import { CodeBlock } from "../ui/CodeBlock";

type Labels = { title: string; warning: string; copy: string; close: string; manifest: string; probes: string };

export function agentManifest(origin: string, cluster: string, secret: string) {
  return [
    "apiVersion: v1",
    "kind: Secret",
    "metadata:",
    "  name: svc-registry-agent",
    "  namespace: svc-registry",
    "stringData:",
    `  token: ${secret}`,
    "---",
    `svc-registry agent --url ${origin} --cluster ${cluster} --token-env SVCR_AGENT_TOKEN --health-addr :8081`,
  ].join("\n");
}

export function AgentTokenDialog({ cluster, secret, labels, onClose }: { cluster: string; secret: string; labels: Labels; onClose: () => void }) {
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  return (
    <KeyDialog secret={secret} labels={labels} onClose={onClose}>
      <CodeBlock className="mt-4" title={labels.manifest} code={agentManifest(origin, cluster, secret)} />
      <p className="mt-2 text-sm text-muted">{labels.probes}</p>
    </KeyDialog>
  );
}
