"use client";

import { AppWindow } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { apiGet, apiSend, type Items, type OAuthApp } from "@/lib/api";
import { appName } from "@/lib/oauthApps";

import { useUiText } from "../UiText";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { SkeletonTable } from "../ui/Skeleton";
import { TokenServersDialog, useGatewayOn } from "../mcp/TokenServersDialog";
import { AppTable } from "./AppTable";
import type { TokenLabels } from "./TokenTable";

type Props = {
  endpoint: `/${string}`;
  locale: Locale;
  labels: TokenLabels;
  empty: string;
  fail: (e: unknown) => string;
};

export function AppList({ endpoint, locale, labels, empty, fail }: Props) {
  const { common } = useUiText();
  const [apps, setApps] = useState<OAuthApp[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [revoking, setRevoking] = useState<OAuthApp | null>(null);
  const [serving, setServing] = useState<OAuthApp | null>(null);
  const gateway = useGatewayOn() && endpoint === "/v1/account/apps";

  const load = useCallback(async () => {
    try {
      setApps((await apiGet<Items<OAuthApp>>(endpoint)).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [endpoint, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  return (
    <>
      {error && <Message note={{ kind: "error", text: error }} className="m-4" />}
      {!apps && !error && <SkeletonTable rows={2} label={common.loading} className="p-5" />}
      {apps && apps.length === 0 && <EmptyState icon={AppWindow} title={empty} className="m-5" />}
      {apps && apps.length > 0 && (
        <div className="[&>div]:rounded-none [&>div]:border-0">
          <AppTable apps={apps} locale={locale} labels={labels} onRevoke={setRevoking} onServers={gateway ? setServing : undefined} />
        </div>
      )}
      <TokenServersDialog<OAuthApp>
        open={serving !== null}
        onOpenChange={(o) => !o && setServing(null)}
        current={serving?.mcp_servers}
        endpoint={serving ? `/v1/account/apps/${serving.id}/mcp-servers` : null}
        labels={labels.gateway}
        fail={fail}
        onSaved={(updated) => {
          setApps((list) => list?.map((x) => (x.id === updated.id ? updated : x)) ?? null);
          toast.success(labels.gateway.saved);
        }}
      />
      {revoking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRevoking(null)}
          title={labels.apps.revokeTitle}
          text={format(labels.apps.confirmRevoke, { name: appName(revoking.client, labels.apps.unnamed) })}
          confirm={labels.revoke}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `${endpoint}/${revoking.id}`);
              toast.success(labels.apps.revoked);
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
