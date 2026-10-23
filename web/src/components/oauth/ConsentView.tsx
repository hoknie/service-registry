"use client";

import { ShieldAlert, ShieldCheck } from "lucide-react";
import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type ConsentRequest } from "@/lib/api";
import { appName, consentError, consentParams, isLoopbackRedirect } from "@/lib/oauthApps";
import { canSubmitNodes, tokenNodesBody, type PickedNode, type TokenAccess } from "@/lib/tokenNodes";

import { NodeScopePicker } from "../account/NodeScopePicker";
import type { NodeScopeLabels } from "../account/TokenNodes";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Skeleton } from "../ui/Skeleton";

type Props = { labels: Messages["oauth"]; scopeLabels: NodeScopeLabels };

type State =
  | { kind: "loading" }
  | { kind: "refused"; text: string }
  | { kind: "ready"; request: ConsentRequest };

export function ConsentView({ labels: t, scopeLabels }: Props) {
  const { errors } = useUiText();
  const [state, setState] = useState<State>({ kind: "loading" });
  const [access, setAccess] = useState<TokenAccess>("all");
  const [nodes, setNodes] = useState<PickedNode[]>([]);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const { request, error } = consentParams(window.location.search);
    const known = consentError(error);
    const controller = new AbortController();
    if (known || !request) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setState({ kind: "refused", text: known ? errorText(errors, known) : t.stale });
      return;
    }
    apiGet<ConsentRequest>(`/v1/oauth/requests/${encodeURIComponent(request)}`, controller.signal)
      .then((r) => setState({ kind: "ready", request: r }))
      .catch((e) => {
        if (controller.signal.aborted) return;
        const code = errorCode(e);
        setState({ kind: "refused", text: code === "not_found" ? t.stale : errorText(errors, code) });
      });
    return () => controller.abort();
  }, [errors, t.stale]);

  if (state.kind === "loading") {
    return (
      <div role="status" className="grid gap-3">
        <span className="sr-only">{t.loading}</span>
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-40" />
      </div>
    );
  }
  if (state.kind === "refused") {
    return (
      <div role="alert">
        <EmptyState icon={ShieldAlert} title={t.title} text={state.text} />
      </div>
    );
  }

  const { request } = state;
  const decide = async (verdict: "approve" | "deny") => {
    setBusy(true);
    setNote(null);
    try {
      const body = verdict === "approve" ? { nodes: tokenNodesBody(access, nodes) } : {};
      const out = await apiSend<{ redirect_to: string }>("POST", `/v1/oauth/requests/${request.id}/${verdict}`, body);
      window.location.assign(out.redirect_to);
    } catch (e) {
      const code = errorCode(e);
      setNote({ kind: "error", text: code === "not_found" ? t.stale : errorText(errors, code) });
      setBusy(false);
    }
  };

  return (
    <Panel title={t.title}>
      <div className="grid gap-5">
        <div className="flex items-start gap-3">
          <ShieldCheck aria-hidden="true" className="mt-1 size-6 shrink-0 text-signal" />
          <div className="min-w-0">
            <p className="text-lg font-semibold break-words text-ink">{appName(request.client, t.unnamed)}</p>
            {!request.client.metadata_url && <p className="text-sm text-amber">{t.unverified}</p>}
          </div>
        </div>
        {request.client.metadata_url && (
          <div className="grid gap-1">
            <p className="text-sm text-muted">{t.publishedOn}</p>
            <p className="text-xl font-semibold break-all text-ink">{request.client.metadata_host}</p>
            <p className="font-mono text-xs break-all text-muted">{request.client.metadata_url}</p>
          </div>
        )}
        {request.client.metadata_url && isLoopbackRedirect(request.client.redirect_uri) && (
          <p role="note" className="text-sm text-amber">
            {t.loopbackWarning}
          </p>
        )}
        <div className="grid gap-1">
          <p className="text-sm text-muted">{t.returnTo}</p>
          <p className="text-xl font-semibold break-all text-ink">{request.client.redirect_host}</p>
          {request.client.redirect_uri && <p className="font-mono text-xs break-all text-muted">{request.client.redirect_uri}</p>}
        </div>
        <div className="grid gap-1">
          <p className="text-sm font-medium text-ink-2">{t.access}</p>
          <p className="text-sm text-ink">{t.accessText}</p>
        </div>
        <NodeScopePicker access={access} onAccess={setAccess} nodes={nodes} onNodes={setNodes} labels={scopeLabels} />
        <Message note={note} />
        <div className="flex flex-wrap justify-end gap-2">
          <Button busy={busy} disabled={busy} onClick={() => void decide("deny")}>
            {t.deny}
          </Button>
          <Button variant="primary" busy={busy} disabled={busy || !canSubmitNodes(access, nodes, false)} onClick={() => void decide("approve")}>
            {t.allow}
          </Button>
        </div>
      </div>
    </Panel>
  );
}
