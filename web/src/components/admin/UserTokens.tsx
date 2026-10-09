"use client";

import { Plus } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { apiGet, apiSend, type Items, type PersonalToken } from "@/lib/api";

import { TokenDialog } from "../account/TokenDialog";
import { TokenTable, type TokenLabels } from "../account/TokenTable";
import { KeyDialog } from "../catalog/KeyDialog";
import { Button } from "../ui/Button";
import { useUiText } from "../UiText";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Message } from "../ui/Message";
import { SkeletonTable } from "../ui/Skeleton";

type Props = {
  userId: string;
  locale: Locale;
  title: string;
  empty: string;
  labels: TokenLabels;
  fail: (e: unknown) => string;
  canIssue?: boolean;
  superadmin?: boolean;
};

export function UserTokens({ userId, locale, title, empty, labels, fail, canIssue, superadmin = false }: Props) {
  const { common } = useUiText();
  const [tokens, setTokens] = useState<PersonalToken[] | null>(null);
  const [issuing, setIssuing] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<PersonalToken | null>(null);

  const load = useCallback(async () => {
    try {
      setTokens((await apiGet<Items<PersonalToken>>(`/v1/users/${userId}/tokens`)).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [userId, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  return (
    <section className="grid gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3>{title}</h3>
        {canIssue && (
          <Button size="sm" variant="primary" onClick={() => setIssuing(true)}>
            <Plus aria-hidden="true" />
            {labels.issue}
          </Button>
        )}
      </div>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!tokens && !error && <SkeletonTable rows={2} label={common.loading} />}
      {tokens && tokens.length === 0 && <p className="text-sm text-muted">{empty}</p>}
      {tokens && tokens.length > 0 && <TokenTable tokens={tokens} locale={locale} labels={labels} onRevoke={setRevoking} />}
      {canIssue && (
        <TokenDialog
          open={issuing}
          onOpenChange={setIssuing}
          superadmin={superadmin}
          labels={labels}
          fail={fail}
          endpoint={`/v1/users/${userId}/tokens`}
          onIssued={(token) => {
            setIssuing(false);
            setSecret(token.secret ?? null);
            toast.success(labels.dialog.done);
            void load();
          }}
        />
      )}
      {secret && <KeyDialog secret={secret} labels={labels.secret} onClose={() => setSecret(null)} />}
      {revoking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRevoking(null)}
          title={labels.revokeTitle}
          text={format(labels.confirmRevoke, { name: revoking.name, prefix: revoking.prefix })}
          confirm={labels.revoke}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/tokens/${revoking.id}`);
              toast.success(labels.revoked);
              await load();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </section>
  );
}
