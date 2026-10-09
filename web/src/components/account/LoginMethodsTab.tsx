"use client";

import { Link2, Unlink } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { when } from "@/i18n/time";
import { apiGet, apiSend, errorCode, oauthStartUrl, type Identity, type Items, type Providers } from "@/lib/api";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { PasswordPanel } from "./PasswordPanel";

type Props = { locale: Locale; labels: Messages["account"] };

export function LoginMethodsTab({ locale, labels }: Props) {
  const { errors, common } = useUiText();
  const { session } = useSession();
  const [identities, setIdentities] = useState<Identity[] | null>(null);
  const [providers, setProviders] = useState<Providers | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [unlinking, setUnlinking] = useState<Identity | null>(null);
  const t = labels.login;

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(async () => {
    try {
      const [own, all] = await Promise.all([
        apiGet<Items<Identity>>("/v1/account/identities"),
        apiGet<Providers>("/v1/providers"),
      ]);
      setIdentities(own.items);
      setProviders(all);
    } catch (e) {
      setError(fail(e));
    }
  }, [fail]);

  useEffect(() => {
    const code = new URLSearchParams(window.location.search).get("error");
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (code) setError(errorText(errors, code));
    void load();
  }, [load, errors]);

  if (session.status !== "signed-in") return null;
  const linked = new Set(identities?.map((i) => i.provider));
  const linkable = providers?.providers.filter((p) => !linked.has(p.key)) ?? [];

  return (
    <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
      <Panel title={t.title} description={t.lead}>
        <dl className="mb-5 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-2 text-sm">
          <dt className="text-muted">{t.password}</dt>
          <dd>
            <Badge tone={session.user.has_password ? "signal" : "outline"}>
              {session.user.has_password ? t.passwordSet : t.passwordNone}
            </Badge>
          </dd>
        </dl>
        {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
        <h3 className="mb-2 text-sm font-semibold text-ink">{t.identities}</h3>
        {!identities && !error && <SkeletonTable rows={2} label={common.loading} />}
        {identities && identities.length === 0 && <p className="text-sm text-muted">{t.identitiesEmpty}</p>}
        {identities && identities.length > 0 && (
          <ul className="grid gap-2">
            {identities.map((i) => (
              <li key={i.id} className="flex items-center gap-3 rounded-lg border border-line px-3 py-2">
                <div className="min-w-0 flex-1">
                  <p className="truncate font-medium text-ink">{i.display_name}</p>
                  <p className="truncate text-xs text-muted">
                    {i.email ?? "—"} · {t.lastLogin}: {when(i.last_login_at, locale, "—")}
                  </p>
                </div>
                <Button variant="ghost" size="sm" onClick={() => setUnlinking(i)}>
                  <Unlink aria-hidden="true" />
                  {t.unlink}
                </Button>
              </li>
            ))}
          </ul>
        )}
        {linkable.length > 0 && (
          <div className="mt-5">
            <p className="mb-2 text-sm text-muted">{t.linkLead}</p>
            <div className="flex flex-wrap gap-2">
              {linkable.map((p) => (
                <Button key={p.key} asChild variant="secondary" size="sm">
                  <a href={oauthStartUrl(p.key, { link: true })}>
                    <Link2 aria-hidden="true" />
                    {format(t.link, { provider: p.display_name })}
                  </a>
                </Button>
              ))}
            </div>
          </div>
        )}
      </Panel>
      {!session.user.has_password && <PasswordPanel labels={labels} />}

      {unlinking && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setUnlinking(null)}
          title={t.unlinkTitle}
          text={format(t.confirmUnlink, { provider: unlinking.display_name, email: unlinking.email ?? "—" })}
          confirm={t.unlink}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/account/identities/${unlinking.id}`);
              toast.success(t.unlinked);
              await load();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </div>
  );
}
