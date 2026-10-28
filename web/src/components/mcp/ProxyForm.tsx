"use client";

import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiSend, errorCode } from "@/lib/api";
import type { McpProxyMode, McpProxySetting } from "@/lib/mcp";
import { loadProxy, proxyUrl, type ProxyOwner } from "@/lib/mcpApi";

import { SecretPicker } from "../secrets/SecretPicker";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";

type Props = {
  owner: ProxyOwner;
  id: string;
  organization: string | null;
  canEdit: boolean;
  locale: Locale;
  labels: Messages["mcp"];
};

export function ProxyForm({
  owner,
  id,
  organization,
  canEdit,
  locale,
  labels: t,
}: Props) {
  const x = t.proxy;
  const { errors } = useUiText();
  const [setting, setSetting] = useState<McpProxySetting | null>(null);
  const [mode, setMode] = useState<McpProxyMode | "">("");
  const [url, setUrl] = useState("");
  const [noProxy, setNoProxy] = useState("");
  const [secret, setSecret] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  const apply = (s: McpProxySetting) => {
    setSetting(s);
    setMode(s.own?.mode ?? "");
    setUrl(s.own?.url ?? "");
    setNoProxy(s.own?.no_proxy ?? "");
    setSecret(s.own?.credentials?.secret.id ?? null);
  };

  const load = useCallback(async () => {
    try {
      apply(await loadProxy(owner, id));
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    }
  }, [owner, id, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      if (mode === "") {
        await apiSend("DELETE", proxyUrl(owner, id));
        await load();
      } else {
        apply(
          await apiSend<McpProxySetting>("PUT", proxyUrl(owner, id), {
            mode,
            url: mode === "proxy" ? url : null,
            no_proxy: mode === "proxy" && noProxy ? noProxy : null,
            credentials:
              mode === "proxy" && secret ? { secret_id: secret } : null,
          }),
        );
      }
      setNote({ kind: "success", text: x.saved });
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const eff = setting?.effective;
  const from = setting?.from;
  return (
    <Panel title={x.title} description={x.lead}>
      <div className="grid gap-4">
        {eff && (
          <p className="text-sm text-ink-2">
            {format(x.effective, { mode: x.modes[eff.mode] })}
            {eff.url ? ` — ${eff.url}` : ""}
            {" · "}
            {from
              ? format(x.from, { kind: x.owners[from.kind], name: from.name })
              : x.fromDefault}
          </p>
        )}
        <Field label={x.mode}>
          {(p) => (
            <Select
              {...p}
              className="w-72"
              disabled={!canEdit}
              value={mode}
              onChange={(e) => setMode(e.target.value as McpProxyMode | "")}
            >
              <option value="">{x.inherit}</option>
              <option value="direct">{x.modes.direct}</option>
              <option value="environment">{x.modes.environment}</option>
              <option value="proxy">{x.modes.proxy}</option>
            </Select>
          )}
        </Field>
        {mode === "proxy" && (
          <>
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={x.url}>
                {(p) => (
                  <Input
                    {...p}
                    disabled={!canEdit}
                    className="font-mono"
                    placeholder="http://proxy.internal:3128"
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                  />
                )}
              </Field>
              <Field label={x.noProxy} hint={x.noProxyHint}>
                {(p) => (
                  <Input
                    {...p}
                    disabled={!canEdit}
                    className="font-mono"
                    value={noProxy}
                    onChange={(e) => setNoProxy(e.target.value)}
                  />
                )}
              </Field>
            </div>
            <SecretPicker
              listOn={organization}
              createOn={false}
              value={secret}
              onChange={setSecret}
              manageHref={
                organization
                  ? `/${locale}/catalog?node=${organization}&tab=settings&section=secrets`
                  : `/${locale}/admin/secrets`
              }
              allowNone
            />
          </>
        )}
        <Message note={note} />
        {canEdit && (
          <div>
            <Button
              variant="primary"
              busy={busy}
              onClick={save}
              disabled={mode === "proxy" && !url}
            >
              {x.save}
            </Button>
          </div>
        )}
      </div>
    </Panel>
  );
}
