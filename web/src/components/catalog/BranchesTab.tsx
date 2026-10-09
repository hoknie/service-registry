"use client";

import { GitBranch, Pin, PinOff, Search, Trash } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { apiGet, apiSend, errorCode, type Branch, type Page } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message } from "../ui/Message";
import { Pager } from "../ui/Pager";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { catalogHref, type CatalogLabels } from "./shared";

const PAGE = 50;
const STATES = ["active", "stale", "gone", "all"] as const;
type State = (typeof STATES)[number];

type Props = { projectId: string; locale: Locale; labels: CatalogLabels; canWrite: boolean };

export function BranchesTab({ projectId, locale, labels, canWrite }: Props) {
  const { errors, common } = useUiText();
  const t = labels.branches;
  const [state, setState] = useState<State>("active");
  const [draft, setDraft] = useState("");
  const [q, setQ] = useState("");
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<Page<Branch> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<Branch | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const base = `/v1/catalog/nodes/${projectId}/branches`;

  const load = useCallback(async () => {
    const query = new URLSearchParams({ state, limit: String(PAGE), offset: String(offset) });
    if (q) query.set("q", q);
    try {
      setPage(await apiGet<Page<Branch>>(`${base}?${query}` as `/${string}`));
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [base, state, q, offset, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const search = (e: FormEvent) => {
    e.preventDefault();
    setOffset(0);
    setQ(draft.trim());
  };

  const pin = async (b: Branch) => {
    try {
      await apiSend(b.pinned ? "DELETE" : "PUT", `${base}/pin?name=${encodeURIComponent(b.name)}` as `/${string}`);
      toast.success(b.pinned ? t.unpinned : t.pinned);
      void load();
    } catch (e) {
      toast.error(fail(e));
    }
  };

  return (
    <Panel title={t.title} description={t.lead}>
      <form className="mb-4 flex flex-wrap items-end gap-2" role="search" onSubmit={search}>
        <Field label={t.search} className="w-64 max-w-full">
          {(p) => <Input {...p} spellCheck={false} className="font-mono" value={draft} onChange={(e) => setDraft(e.target.value)} />}
        </Field>
        <Button type="submit">
          <Search aria-hidden="true" />
          {t.searchSubmit}
        </Button>
        <Field label={t.state} className="w-44">
          {(p) => (
            <Select
              {...p}
              value={state}
              onChange={(e) => {
                setOffset(0);
                setState(e.target.value as State);
              }}
            >
              {STATES.map((s) => (
                <option key={s} value={s}>
                  {t.states[s]}
                </option>
              ))}
            </Select>
          )}
        </Field>
      </form>
      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
      {!page && !error && <SkeletonTable rows={5} label={common.loading} />}
      {page && page.items.length === 0 && <EmptyState icon={GitBranch} title={t.empty} />}
      {page && page.items.length > 0 && (
        <>
          <Table>
            <thead>
              <tr>
                <Th>{t.table.name}</Th>
                <Th>{t.table.head}</Th>
                <Th>{t.table.sources}</Th>
                <Th>{t.table.activity}</Th>
                {canWrite && (
                  <Th>
                    <span className="sr-only">{t.table.actions}</span>
                  </Th>
                )}
              </tr>
            </thead>
            <tbody>
              {page.items.map((b) => (
                <Tr key={b.name}>
                  <Td>
                    <Link href={catalogHref(locale, projectId, "deployments", b.is_default ? undefined : b.name)} className="font-mono font-medium text-ink hover:underline">
                      {b.name}
                    </Link>
                    <span className="mt-1 flex flex-wrap gap-1">
                      {b.is_default && <Badge tone="signal">{t.badges.default}</Badge>}
                      {b.pinned && <Badge tone="outline">{t.badges.pinned}</Badge>}
                      {b.protected && <Badge tone="outline">{t.badges.protected}</Badge>}
                      {b.stale && !b.gone_at && <Badge tone="amber">{t.badges.stale}</Badge>}
                      {b.gone_at && <Badge tone="danger">{t.badges.gone}</Badge>}
                    </span>
                  </Td>
                  <Td>{b.head_sha ? <code className="text-ink-2">{b.head_sha.slice(0, 7)}</code> : <span className="text-muted">—</span>}</Td>
                  <Td className="text-ink-2">{b.sources.map((s) => t.sources[s]).join(", ")}</Td>
                  <Td className="whitespace-nowrap text-ink-2" title={when(b.last_activity_at, locale, "")}>
                    {ago(b.last_activity_at, locale, t.never)}
                  </Td>
                  {canWrite && (
                    <Td className="text-right whitespace-nowrap">
                      <Button size="sm" variant="ghost" onClick={() => void pin(b)}>
                        {b.pinned ? <PinOff aria-hidden="true" /> : <Pin aria-hidden="true" />}
                        {b.pinned ? t.unpin : t.pin}
                      </Button>
                      {!b.is_default && (
                        <Button size="sm" variant="danger-ghost" onClick={() => setDeleting(b)}>
                          <Trash aria-hidden="true" />
                          {t.delete}
                        </Button>
                      )}
                    </Td>
                  )}
                </Tr>
              ))}
            </tbody>
          </Table>
          <Pager offset={offset} shown={page.items.length} total={page.total} size={PAGE} labels={labels.pager} onChange={setOffset} />
        </>
      )}
      {deleting && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setDeleting(null)}
          title={t.deleteTitle}
          text={format(t.confirmDelete, { name: deleting.name })}
          confirm={t.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `${base}/item?name=${encodeURIComponent(deleting.name)}` as `/${string}`);
              toast.success(t.deleted);
              void load();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </Panel>
  );
}
