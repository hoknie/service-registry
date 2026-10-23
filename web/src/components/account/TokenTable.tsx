"use client";

import Link from "next/link";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import type { PersonalToken } from "@/lib/api";
import { tokenNodeLabels } from "@/lib/tokenNodes";

import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Table, Td, Th, Tr } from "../ui/Table";

export type TokenLabels = Messages["tokens"];

const statusTone: Record<PersonalToken["status"], Tone> = { active: "signal", expired: "neutral", revoked: "danger" };

type Props = {
  tokens: PersonalToken[];
  locale: Locale;
  labels: TokenLabels;
  showOwner?: boolean;
  onRevoke: (token: PersonalToken) => void;
};

export function TokenTable({ tokens, locale, labels, showOwner, onRevoke }: Props) {
  const t = labels.table;
  return (
    <Table>
      <thead>
        <tr>
          <Th>{t.name}</Th>
          {showOwner && <Th>{t.owner}</Th>}
          <Th>{t.scopes}</Th>
          <Th>{t.nodes}</Th>
          <Th>{t.created}</Th>
          <Th>{t.expires}</Th>
          <Th>{t.lastUsed}</Th>
          <Th>{t.status}</Th>
          <Th>
            <span className="sr-only">{t.actions}</span>
          </Th>
        </tr>
      </thead>
      <tbody>
        {tokens.map((tok) => (
          <Tr key={tok.id}>
            <Td className="min-w-48">
              <span className="block font-medium break-words text-ink">{tok.name}</span>
              <code className="block text-xs text-muted" title={t.prefix}>
                {tok.prefix}…
              </code>
            </Td>
            {showOwner && (
              <Td className="text-ink-2">
                {tok.user_id ? (
                  <Link href={`/${locale}/admin/users/user?id=${tok.user_id}`} className="hover:text-signal hover:underline">
                    {tok.user_email}
                  </Link>
                ) : (
                  tok.user_email
                )}
              </Td>
            )}
            <Td>
              <span className="flex flex-wrap gap-1">
                {tok.scopes.map((s) => (
                  <Badge key={s} tone={s === "admin" ? "amber" : "neutral"} title={labels.scopeHints[s]}>
                    {s}
                  </Badge>
                ))}
                {tok.catalog_admin && !tok.scopes.includes("admin") && (
                  <Badge tone="amber" title={labels.catalogAdminHint}>
                    {labels.catalogAdmin}
                  </Badge>
                )}
              </span>
            </Td>
            <Td className="min-w-32 text-ink-2">
              <span className="grid gap-0.5">
                {tokenNodeLabels(tok, labels.allNodes, labels.deletedNode).map((label, i) => (
                  <span key={`${label}-${i}`} className={tok.nodes && tok.nodes[i]?.path ? "font-mono text-xs break-all" : undefined}>
                    {label}
                  </span>
                ))}
              </span>
            </Td>
            <Td className="whitespace-nowrap text-ink-2" title={when(tok.created_at, locale, "")}>
              {ago(tok.created_at, locale, "")}
            </Td>
            <Td className="whitespace-nowrap text-ink-2">{when(tok.expires_at, locale, labels.noExpiry)}</Td>
            <Td className="whitespace-nowrap text-ink-2" title={when(tok.last_used_at, locale, "")}>
              {ago(tok.last_used_at, locale, labels.never)}
            </Td>
            <Td>
              <Badge tone={statusTone[tok.status]} dot>
                {labels.statuses[tok.status]}
              </Badge>
            </Td>
            <Td className="text-right">
              {tok.status === "active" && (
                <Button size="sm" variant="danger-ghost" onClick={() => onRevoke(tok)}>
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
