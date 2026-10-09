"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { apiGet, apiSend, type Items, type PersonalToken } from "@/lib/api";

import { TokenTable, type TokenLabels } from "../account/TokenTable";
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
};

export function UserTokens({ userId, locale, title, empty, labels, fail }: Props) {
  const { common } = useUiText();
  const [tokens, setTokens] = useState<PersonalToken[] | null>(null);
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
    <section className="mt-6 grid gap-3 border-t border-line pt-5">
      <h3>{title}</h3>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!tokens && !error && <SkeletonTable rows={2} label={common.loading} />}
      {tokens && tokens.length === 0 && <p className="text-sm text-muted">{empty}</p>}
      {tokens && tokens.length > 0 && <TokenTable tokens={tokens} locale={locale} labels={labels} onRevoke={setRevoking} />}
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
