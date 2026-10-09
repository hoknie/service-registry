"use client";

import { KeyRound, Plus } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type Items, type PersonalToken } from "@/lib/api";

import { KeyDialog } from "../catalog/KeyDialog";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { McpCommand, McpConnect } from "./McpConnect";
import { TokenDialog } from "./TokenDialog";
import { TokenTable, type TokenLabels } from "./TokenTable";

type Props = { locale: Locale; superadmin: boolean; labels: TokenLabels };

export function TokensTab({ locale, superadmin, labels }: Props) {
  const { errors, common } = useUiText();
  const [tokens, setTokens] = useState<PersonalToken[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [issuing, setIssuing] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);
  const [mcpIssuing, setMcpIssuing] = useState(false);
  const [mcpSecret, setMcpSecret] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<PersonalToken | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(async () => {
    try {
      setTokens((await apiGet<Items<PersonalToken>>("/v1/account/tokens")).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const issue = (
    <Button variant="primary" onClick={() => setIssuing(true)}>
      <Plus aria-hidden="true" />
      {labels.issue}
    </Button>
  );

  return (
    <>
      <Panel title={labels.title} description={labels.lead} actions={issue} bodyClassName="p-0 sm:p-0">
        {error && <Message note={{ kind: "error", text: error }} className="m-4" />}
        {!tokens && !error && <SkeletonTable rows={3} label={common.loading} className="p-5" />}
        {tokens && tokens.length === 0 && <EmptyState icon={KeyRound} title={labels.empty} className="m-5" />}
        {tokens && tokens.length > 0 && (
          <div className="[&>div]:rounded-none [&>div]:border-0">
            <TokenTable tokens={tokens} locale={locale} labels={labels} onRevoke={setRevoking} />
          </div>
        )}
      </Panel>

      <TokenDialog
        open={issuing}
        onOpenChange={setIssuing}
        superadmin={superadmin}
        labels={labels}
        fail={fail}
        onIssued={(token) => {
          setIssuing(false);
          setSecret(token.secret ?? null);
          toast.success(labels.dialog.done);
          void load();
        }}
      />
      {secret && <KeyDialog secret={secret} labels={labels.secret} onClose={() => setSecret(null)} />}

      <div className="mt-6">
        <McpConnect labels={labels.mcp} onIssue={() => setMcpIssuing(true)} />
      </div>
      <TokenDialog
        open={mcpIssuing}
        onOpenChange={setMcpIssuing}
        superadmin={superadmin}
        labels={labels}
        fail={fail}
        presetScopes={["mcp"]}
        onIssued={(token) => {
          setMcpIssuing(false);
          setMcpSecret(token.secret ?? null);
          toast.success(labels.dialog.done);
          void load();
        }}
      />
      {mcpSecret && (
        <KeyDialog secret={mcpSecret} labels={labels.secret} onClose={() => setMcpSecret(null)}>
          <McpCommand token={mcpSecret} labels={labels.mcp} />
        </KeyDialog>
      )}
      {revoking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRevoking(null)}
          title={labels.revokeTitle}
          text={format(labels.confirmRevoke, { name: revoking.name, prefix: revoking.prefix })}
          confirm={labels.revoke}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/account/tokens/${revoking.id}`);
              toast.success(labels.revoked);
              await load();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </>
  );
}
