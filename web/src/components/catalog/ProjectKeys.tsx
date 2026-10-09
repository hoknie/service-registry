"use client";

import { KeyRound, Plus } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { apiGet, apiSend, errorCode, type Items, type ProjectKey } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Field } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { KeyDialog } from "./KeyDialog";
import type { CatalogLabels } from "./shared";

const GRACE = { none: 0, hour: 3600, day: 86_400, week: 604_800 } as const;
type Grace = keyof typeof GRACE;
const statusTone: Record<ProjectKey["status"], Tone> = { active: "signal", expired: "neutral", revoked: "danger" };

type Props = { projectId: string; locale: Locale; labels: CatalogLabels };

export function ProjectKeys({ projectId, locale, labels }: Props) {
  const { errors, common } = useUiText();
  const t = labels.keys;
  const [keys, setKeys] = useState<ProjectKey[] | null>(null);
  const [grace, setGrace] = useState<Grace>("day");
  const [secret, setSecret] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<ProjectKey | null>(null);
  const [busy, setBusy] = useState(false);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(async () => {
    try {
      setKeys((await apiGet<Items<ProjectKey>>(`/v1/catalog/nodes/${projectId}/keys`)).items);
    } catch (e) {
      setError(fail(e));
    }
  }, [projectId, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const issue = async () => {
    setBusy(true);
    try {
      const key = await apiSend<ProjectKey>("POST", `/v1/catalog/nodes/${projectId}/keys`, { grace_secs: GRACE[grace] });
      setSecret(key.secret ?? null);
      setError(null);
      await load();
    } catch (e) {
      toast.error(fail(e));
    } finally {
      setBusy(false);
    }
  };

  const hasActive = keys?.some((k) => k.status === "active") ?? false;

  return (
    <div className="grid gap-6">
      <Panel
        title={t.title}
        description={t.lead}
        actions={
          <div className="flex flex-wrap items-end gap-2">
            {hasActive && (
              <Field label={t.grace} className="w-60">
                {(p) => (
                  <Select {...p} value={grace} onChange={(e) => setGrace(e.target.value as Grace)}>
                    {(Object.keys(GRACE) as Grace[]).map((g) => (
                      <option key={g} value={g}>
                        {t.graceOptions[g]}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
            )}
            <Button variant="primary" disabled={busy} onClick={() => void issue()}>
              <Plus aria-hidden="true" />
              {t.issue}
            </Button>
          </div>
        }
        bodyClassName="p-0 sm:p-0"
      >
        {error && <Message note={{ kind: "error", text: error }} className="m-4" />}
        {!keys && !error && <SkeletonTable rows={3} label={common.loading} className="p-5" />}
        {keys && keys.length === 0 && <EmptyState icon={KeyRound} title={t.empty} className="m-5" />}
        {keys && keys.length > 0 && (
          <div className="[&>div]:rounded-none [&>div]:border-0">
            <Table>
              <thead>
                <tr>
                  <Th>{t.prefix}</Th>
                  <Th>{t.status}</Th>
                  <Th>{t.created}</Th>
                  <Th>{t.lastUsed}</Th>
                  <Th>{t.expires}</Th>
                  <Th>
                    <span className="sr-only">{t.revoke}</span>
                  </Th>
                </tr>
              </thead>
              <tbody>
                {keys.map((k) => (
                  <Tr key={k.id}>
                    <Td>
                      <code className="font-medium text-ink">{k.prefix}…</code>
                    </Td>
                    <Td>
                      <Badge tone={statusTone[k.status]} dot>
                        {t.statuses[k.status]}
                      </Badge>
                    </Td>
                    <Td className="whitespace-nowrap text-ink-2" title={when(k.created_at, locale, "")}>
                      {ago(k.created_at, locale, t.never)}
                    </Td>
                    <Td className="whitespace-nowrap text-ink-2" title={when(k.last_used_at, locale, "")}>
                      {ago(k.last_used_at, locale, t.never)}
                    </Td>
                    <Td className="whitespace-nowrap text-ink-2">{when(k.expires_at ?? k.revoked_at, locale, t.never)}</Td>
                    <Td className="text-right">
                      {k.status !== "revoked" && (
                        <Button size="sm" variant="danger-ghost" onClick={() => setRevoking(k)}>
                          {t.revoke}
                        </Button>
                      )}
                    </Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          </div>
        )}
      </Panel>

      {revoking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRevoking(null)}
          title={t.revokeTitle}
          text={format(t.confirmRevoke, { prefix: revoking.prefix })}
          confirm={t.revoke}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/catalog/nodes/${projectId}/keys/${revoking.id}`);
              toast.success(t.revoked);
              await load();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
      {secret && <KeyDialog secret={secret} labels={t.dialog} onClose={() => setSecret(null)} />}
    </div>
  );
}
