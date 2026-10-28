"use client";

import type { Locale } from "@/i18n/config";
import { ago, when } from "@/i18n/time";
import type { OAuthApp } from "@/lib/api";
import { appHost, appName } from "@/lib/oauthApps";
import { tokenNodeLabels } from "@/lib/tokenNodes";

import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { serversLabel } from "../mcp/TokenServersDialog";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { TokenLabels } from "./TokenTable";

const statusTone: Record<OAuthApp["status"], Tone> = { active: "signal", expired: "neutral", revoked: "danger" };

type Props = { apps: OAuthApp[]; locale: Locale; labels: TokenLabels; onRevoke: (app: OAuthApp) => void; onServers?: (app: OAuthApp) => void };

export function AppTable({ apps, locale, labels, onRevoke, onServers }: Props) {
  const t = labels.apps.table;
  return (
    <Table>
      <thead>
        <tr>
          <Th>{t.app}</Th>
          <Th>{t.access}</Th>
          {onServers && <Th>{labels.gateway.column}</Th>}
          <Th>{t.granted}</Th>
          <Th>{t.lastUsed}</Th>
          <Th>{t.expires}</Th>
          <Th>{t.status}</Th>
          <Th>
            <span className="sr-only">{labels.table.actions}</span>
          </Th>
        </tr>
      </thead>
      <tbody>
        {apps.map((app) => (
          <Tr key={app.id}>
            <Td className="min-w-48">
              <span className="block font-medium break-words text-ink">{appName(app.client, labels.apps.unnamed)}</span>
              <span className="block text-xs break-all text-muted">{appHost(app.client)}</span>
            </Td>
            <Td className="min-w-32 text-ink-2">
              <span className="grid gap-0.5">
                {tokenNodeLabels(app, labels.allNodes, labels.deletedNode).map((label, i) => (
                  <span key={`${label}-${i}`} className={app.nodes && app.nodes[i]?.path ? "font-mono text-xs break-all" : undefined}>
                    {label}
                  </span>
                ))}
              </span>
            </Td>
            {onServers && <Td className="min-w-28 text-ink-2">{serversLabel(app.mcp_servers, labels.gateway.allShort, labels.gateway.noneShort)}</Td>}
            <Td className="whitespace-nowrap text-ink-2" title={when(app.created_at, locale, "")}>
              {ago(app.created_at, locale, "")}
            </Td>
            <Td className="whitespace-nowrap text-ink-2" title={when(app.last_used_at, locale, "")}>
              {ago(app.last_used_at, locale, labels.never)}
            </Td>
            <Td className="whitespace-nowrap text-ink-2">{when(app.expires_at, locale, "")}</Td>
            <Td>
              <Badge tone={statusTone[app.status]} dot>
                {labels.apps.statuses[app.status]}
              </Badge>
            </Td>
            <Td className="text-right">
              {app.status === "active" && onServers && (
                <Button size="sm" variant="ghost" onClick={() => onServers(app)}>
                  {labels.gateway.change}
                </Button>
              )}
              {app.status === "active" && (
                <Button size="sm" variant="danger-ghost" onClick={() => onRevoke(app)}>
                  {labels.revoke}
                </Button>
              )}
            </Td>
          </Tr>
        ))}
      </tbody>
    </Table>
  );
}
