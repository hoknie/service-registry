"use client";

import { Activity, Boxes, Pencil, Plug, Plus, RefreshCw, Server, Trash, Unlink } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { apiGet, apiSend, errorCode, type Cluster, type ClusterTest, type DirectoryEnvironment, type Items } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { validDnsLabel } from "@/lib/tags";

import { Checkbox, Field, Input, Textarea } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { hasInvalidTags, TagInput } from "../ui/TagInput";
import { UnmatchedDialog } from "./UnmatchedDialog";

type Labels = Messages["admin"]["clusters"];
type Props = { locale: Locale; labels: Labels; pager: Messages["admin"]["users"]["pager"] };

type Draft = {
  id: string | null;
  name: string;
  environment: string;
  inCluster: boolean;
  apiUrl: string;
  ca: string;
  token: string;
  tokenRef: string;
  namespaces: string;
  rules: string;
  interval: string;
  enabled: boolean;
};

const blank: Draft = {
  id: null,
  name: "",
  environment: "production",
  inCluster: false,
  apiUrl: "",
  ca: "",
  token: "",
  tokenRef: "",
  namespaces: "",
  rules: "",
  interval: "60",
  enabled: true,
};

const statusTone = { ok: "signal", error: "danger", never: "outline" } as const;

const parseRules = (text: string) =>
  text
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean)
    .map((l) => {
      const at = l.indexOf("=");
      return at < 0 ? { label: l, project: "" } : { label: l.slice(0, at).trim(), project: l.slice(at + 1).trim() };
    });
const rulesText = (rules: Cluster["rules"]) => rules.map((r) => `${r.label}=${r.project}`).join("\n");

export function ClustersAdmin({ locale, labels: t, pager }: Props) {
  const { errors, common } = useUiText();
  const [items, setItems] = useState<Cluster[] | null>(null);
  const [directory, setDirectory] = useState<DirectoryEnvironment[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [deleting, setDeleting] = useState<Cluster | null>(null);
  const [testing, setTesting] = useState<{ cluster: Cluster; result: ClusterTest | null } | null>(null);
  const [unmatched, setUnmatched] = useState<Cluster | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(async () => {
    try {
      setItems((await apiGet<Items<Cluster>>("/v1/clusters")).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
    apiGet<Items<DirectoryEnvironment>>("/v1/environments")
      .then((r) => setDirectory(r.items))
      .catch(() => setDirectory([]));
  }, [load]);

  const envName = (key: string) => directory.find((e) => e.key === key)?.names[locale] ?? key;

  const open = (c?: Cluster) => {
    setNote(null);
    setDraft(
      c
        ? {
            id: c.id,
            name: c.name,
            environment: c.environment,
            inCluster: c.in_cluster,
            apiUrl: c.api_url ?? "",
            ca: c.ca_pem ?? "",
            token: "",
            tokenRef: c.credentials?.kind === "ref" ? c.credentials.ref : "",
            namespaces: c.namespaces.join("\n"),
            rules: rulesText(c.rules),
            interval: String(c.interval_secs),
            enabled: c.enabled,
          }
        : blank,
    );
  };

  const save = async () => {
    if (!draft) return;
    setBusy(true);
    setNote(null);
    const interval = Number(draft.interval);
    const body: Record<string, unknown> = {
      name: draft.name,
      environment: draft.environment,
      in_cluster: draft.inCluster,
      namespaces: draft.namespaces.split("\n").map((n) => n.trim()).filter(Boolean),
      rules: parseRules(draft.rules),
      interval_secs: Number.isInteger(interval) ? interval : 0,
      enabled: draft.enabled,
    };
    if (!draft.inCluster) {
      body.api_url = draft.apiUrl;
      body.ca_pem = draft.ca;
      const current = items?.find((c) => c.id === draft.id)?.credentials;
      const ref = draft.tokenRef.trim();
      if (draft.token.trim()) body.credentials = { token: draft.token.trim() };
      else if (ref && !(current?.kind === "ref" && current.ref === ref)) body.credentials = { token_ref: ref };
    }
    try {
      if (draft.id) {
        await apiSend("PATCH", `/v1/clusters/${draft.id}`, body);
        toast.success(t.saved);
      } else {
        await apiSend("POST", "/v1/clusters", body);
        toast.success(t.created);
      }
      setDraft(null);
      void load();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  const test = async (c: Cluster) => {
    setTesting({ cluster: c, result: null });
    try {
      const result = await apiSend<ClusterTest>("POST", `/v1/clusters/${c.id}/test`);
      setTesting({ cluster: c, result });
    } catch (e) {
      setTesting(null);
      toast.error(fail(e));
    }
  };

  const poll = async (c: Cluster) => {
    try {
      await apiSend("POST", `/v1/clusters/${c.id}/poll`);
      toast.success(t.polled);
    } catch (e) {
      toast.error(fail(e));
    }
  };

  return (
    <>
      <PageHeader glyph={Server}
        title={t.title}
        lead={t.lead}
        actions={
          <Button variant="primary" onClick={() => open()}>
            <Plus aria-hidden="true" />
            {t.add}
          </Button>
        }
      />
      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
      {!items && !error && <SkeletonTable rows={3} label={common.loading} />}
      {items && items.length === 0 && <EmptyState icon={Boxes} title={t.empty} />}
      {items && items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.table.name}</Th>
              <Th>{t.table.environment}</Th>
              <Th className="hidden md:table-cell">{t.table.address}</Th>
              <Th>{t.table.status}</Th>
              <Th numeric>{t.table.workloads}</Th>
              <Th>
                <span className="sr-only">{t.table.actions}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((c) => (
              <Tr key={c.id}>
                <Td>
                  <span className="font-medium text-ink">{c.name}</span>
                  {!c.enabled && (
                    <Badge tone="outline" className="ml-2">
                      {t.disabled}
                    </Badge>
                  )}
                </Td>
                <Td className="text-ink-2" title={c.environment}>
                  {envName(c.environment)}
                </Td>
                <Td className="hidden max-w-64 truncate md:table-cell">
                  {c.in_cluster ? <span className="text-ink-2">{t.inCluster}</span> : <code className="text-xs text-ink-2">{c.api_url}</code>}
                </Td>
                <Td>
                  <span className="flex flex-col gap-1">
                    <Badge tone={statusTone[c.status]} dot className="self-start">
                      {t.status[c.status]}
                    </Badge>
                    <span className="text-xs text-muted" title={when(c.last_polled_at, locale, "")}>
                      {c.last_polled_at ? ago(c.last_polled_at, locale, "") : t.never}
                    </span>
                    {c.last_error && <span className="text-xs text-danger">{errorText(errors, c.last_error.code)}</span>}
                  </span>
                </Td>
                <Td numeric className="text-ink-2">
                  {c.workloads}
                  {c.unmatched > 0 && (
                    <Button size="sm" variant="ghost" className="ml-1" onClick={() => setUnmatched(c)}>
                      <Unlink aria-hidden="true" />
                      {t.unmatched}: {c.unmatched}
                    </Button>
                  )}
                </Td>
                <Td className="text-right whitespace-nowrap">
                  <Button size="sm" variant="ghost" onClick={() => void test(c)}>
                    <Plug aria-hidden="true" />
                    {t.test}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => void poll(c)}>
                    <RefreshCw aria-hidden="true" />
                    {t.poll}
                  </Button>
                  <Button size="sm" variant="ghost" onClick={() => open(c)}>
                    <Pencil aria-hidden="true" />
                    {t.edit}
                  </Button>
                  <Button size="sm" variant="danger-ghost" onClick={() => setDeleting(c)}>
                    <Trash aria-hidden="true" />
                    {t.delete}
                  </Button>
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
      {draft && (
        <Dialog
          open
          size="lg"
          onOpenChange={(o) => !o && setDraft(null)}
          title={draft.id ? format(t.editTitle, { name: draft.name }) : t.createTitle}
          footer={
            <>
              <Button onClick={() => setDraft(null)}>{common.cancel}</Button>
              <Button variant="primary" busy={busy} disabled={hasInvalidTags(draft.namespaces, (tag) => (validDnsLabel(tag) ? null : "x"))} onClick={() => void save()}>
                {t.submit}
              </Button>
            </>
          }
        >
          <div className="grid gap-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t.fields.name}>{(p) => <Input {...p} required value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />}</Field>
              <Field label={t.fields.environment} hint={t.fields.environmentHint}>
                {(p) => (
                  <>
                    <Input {...p} list="cluster-environments" className="font-mono" value={draft.environment} onChange={(e) => setDraft({ ...draft, environment: e.target.value })} />
                    <datalist id="cluster-environments">
                      {directory.map((e) => (
                        <option key={e.key} value={e.key}>
                          {e.names[locale]}
                        </option>
                      ))}
                    </datalist>
                  </>
                )}
              </Field>
            </div>
            <Checkbox label={t.fields.inCluster} checked={draft.inCluster} onChange={(e) => setDraft({ ...draft, inCluster: e.target.checked })} />
            {!draft.inCluster && (
              <>
                <Field label={t.fields.apiUrl}>
                  {(p) => <Input {...p} type="url" placeholder="https://k8s.example:6443" value={draft.apiUrl} onChange={(e) => setDraft({ ...draft, apiUrl: e.target.value })} />}
                </Field>
                <Field label={t.fields.ca} hint={t.fields.caHint}>
                  {(p) => <Textarea {...p} rows={3} spellCheck={false} className="font-mono text-xs" value={draft.ca} onChange={(e) => setDraft({ ...draft, ca: e.target.value })} />}
                </Field>
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field label={t.fields.token} hint={t.fields.tokenHint}>
                    {(p) => <Input {...p} type="password" autoComplete="off" value={draft.token} onChange={(e) => setDraft({ ...draft, token: e.target.value })} />}
                  </Field>
                  <Field label={t.fields.tokenRef}>
                    {(p) => <Input {...p} spellCheck={false} className="font-mono" placeholder="env:K8S_TOKEN" value={draft.tokenRef} onChange={(e) => setDraft({ ...draft, tokenRef: e.target.value })} />}
                  </Field>
                </div>
              </>
            )}
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t.fields.namespaces} hint={t.fields.namespacesHint}>
                {(p) => (
                  <TagInput
                    {...p}
                    value={draft.namespaces}
                    onChange={(namespaces) => setDraft({ ...draft, namespaces })}
                    validate={(tag) => (validDnsLabel(tag) ? null : common.invalidName)}
                  />
                )}
              </Field>
              <Field label={t.fields.rules} hint={t.fields.rulesHint}>
                {(p) => <Textarea {...p} rows={3} spellCheck={false} className="font-mono text-xs" value={draft.rules} onChange={(e) => setDraft({ ...draft, rules: e.target.value })} />}
              </Field>
            </div>
            <div className="flex flex-wrap items-end gap-6">
              <Field label={t.fields.interval} className="w-48">
                {(p) => <Input {...p} type="number" min={15} max={3600} value={draft.interval} onChange={(e) => setDraft({ ...draft, interval: e.target.value })} />}
              </Field>
              <Checkbox label={t.fields.enabled} checked={draft.enabled} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} />
            </div>
            {note && <Message note={note} />}
          </div>
        </Dialog>
      )}
      {testing && (
        <Dialog open size="sm" onOpenChange={(o) => !o && setTesting(null)} title={format(t.testTitle, { name: testing.cluster.name })}>
          {!testing.result && <SkeletonTable rows={2} label={common.loading} />}
          {testing.result && (
            <div className="grid gap-2 text-sm">
              {testing.result.version && (
                <p className="flex items-center gap-2 text-ink">
                  <Activity aria-hidden="true" className="size-4 text-signal" />
                  {format(t.testOk, { version: testing.result.version })}
                </p>
              )}
              {testing.result.error && <Message note={{ kind: "error", text: errorText(errors, testing.result.error.code) }} />}
              {testing.result.ok &&
                (testing.result.missing.length > 0 ? (
                  <p className="text-danger">{format(t.testMissing, { list: testing.result.missing.join(", ") })}</p>
                ) : (
                  <p className="text-ink-2">{t.testAllRights}</p>
                ))}
            </div>
          )}
        </Dialog>
      )}
      {unmatched && <UnmatchedDialog cluster={unmatched} locale={locale} labels={t} pager={pager} onClose={() => setUnmatched(null)} />}
      {deleting && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setDeleting(null)}
          title={t.deleteTitle}
          text={format(t.confirmDelete, { name: deleting.name })}
          confirm={t.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/clusters/${deleting.id}`);
              toast.success(t.deleted);
              void load();
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
