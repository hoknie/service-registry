"use client";

import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { apiGet, errorCode, type Cluster, type Page, type UnmatchedWorkload } from "@/lib/api";

import { useUiText } from "../UiText";
import { Dialog } from "../ui/Dialog";
import { Message } from "../ui/Message";
import { Pager } from "../ui/Pager";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";

const PAGE = 50;

type Props = {
  cluster: Cluster;
  locale: Locale;
  labels: Messages["admin"]["clusters"];
  pager: Messages["admin"]["users"]["pager"];
  onClose: () => void;
};

export function UnmatchedDialog({ cluster, locale, labels: t, pager, onClose }: Props) {
  const { errors, common } = useUiText();
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<Page<UnmatchedWorkload> | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setPage(await apiGet<Page<UnmatchedWorkload>>(`/v1/clusters/${cluster.id}/unmatched?limit=${PAGE}&offset=${offset}`));
      setError(null);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  }, [cluster.id, offset, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()} size="lg" title={format(t.unmatchedTitle, { name: cluster.name })} description={t.unmatchedHint}>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!page && !error && <SkeletonTable rows={4} label={common.loading} />}
      {page && page.items.length === 0 && <p className="text-sm text-muted">{t.unmatchedEmpty}</p>}
      {page && page.items.length > 0 && (
        <>
          <Table>
            <thead>
              <tr>
                <Th>{t.unmatchedTable.workload}</Th>
                <Th>{t.unmatchedTable.reason}</Th>
                <Th>{t.unmatchedTable.annotation}</Th>
                <Th>{t.unmatchedTable.observed}</Th>
              </tr>
            </thead>
            <tbody>
              {page.items.map((w) => (
                <Tr key={`${w.namespace}/${w.kind}/${w.name}`}>
                  <Td>
                    <span className="block font-mono text-sm text-ink">
                      {w.namespace}/{w.name}
                    </span>
                    <span className="text-xs text-muted">{w.kind}</span>
                  </Td>
                  <Td className="text-ink-2">{t.reasons[w.reason]}</Td>
                  <Td>{w.annotation ? <code className="text-xs break-all text-ink-2">{w.annotation}</code> : <span className="text-muted">—</span>}</Td>
                  <Td className="whitespace-nowrap text-ink-2" title={when(w.observed_at, locale, "")}>
                    {ago(w.observed_at, locale, "")}
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
          <Pager offset={offset} shown={page.items.length} total={page.total} size={PAGE} labels={pager} onChange={setOffset} />
        </>
      )}
    </Dialog>
  );
}
