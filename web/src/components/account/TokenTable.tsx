"use client";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import type { PersonalToken } from "@/lib/api";

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
          <Th>{t.prefix}</Th>
          <Th>{t.scopes}</Th>
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
            <Td className="font-medium text-ink">{tok.name}</Td>
            {showOwner && <Td className="text-ink-2">{tok.user_email}</Td>}
            <Td>
              <code className="text-ink">{tok.prefix}…</code>
            </Td>
            <Td>
              <span className="flex flex-wrap gap-1">
                {tok.scopes.map((s) => (
                  <Badge key={s} tone={s === "admin" ? "amber" : "neutral"} title={labels.scopeHints[s]}>
                    {s}
                  </Badge>
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
