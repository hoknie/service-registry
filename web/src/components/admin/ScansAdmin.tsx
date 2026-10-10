"use client";

import { BookOpen, CheckCircle2, ChevronDown, CircleMinus, ScanSearch, TriangleAlert, XCircle } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Fragment, useEffect, useMemo, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { apiGet, errorCode, type Page, type Scan, type ScanStatus } from "@/lib/api";
import { useCatalogTree } from "@/lib/catalogTree";
import { index, path } from "@/lib/catalogNav";
import { cn } from "@/lib/cn";
import { shortCommit } from "@/lib/commit";
import {
  formatDuration,
  readScanParams,
  SCAN_PAGE,
  SCAN_STATUSES,
  scanAddress,
  scanCodes,
  scanCodeText,
  scanQuery,
  scanTotals,
  seriesText,
  type CodeBook,
  type ScanParams,
} from "@/lib/scans";

import { catalogHref } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Combobox } from "../ui/Combobox";
import { EmptyState } from "../ui/EmptyState";
import { Field } from "../ui/Field";
import { Message } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { Pager } from "../ui/Pager";
import { Select } from "../ui/Select";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";

type Labels = Messages["scans"];

const tones: Record<ScanStatus, Tone> = { ok: "signal", unchanged: "neutral", warning: "amber", failed: "danger" };
const icons = { ok: CheckCircle2, unchanged: CircleMinus, warning: TriangleAlert, failed: XCircle } as const;

export function ScansAdmin({ locale, labels: t }: { locale: Locale; labels: Labels }) {
  const { errors, common } = useUiText();
  const router = useRouter();
  const search = useSearchParams();
  const params = readScanParams((n) => search.get(n));
  const query = scanQuery(params);
  const [page, setPage] = useState<{ query: string; data: Page<Scan> } | null>(null);
  const [failure, setFailure] = useState<{ query: string; text: string } | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const tree = useCatalogTree();

  useEffect(() => {
    let alive = true;
    apiGet<Page<Scan>>(`/v1/knowledge/scans?${query}`).then(
      (data) => alive && setPage({ query, data }),
      (e: unknown) => alive && setFailure({ query, text: errorText(errors, errorCode(e)) }),
    );
    return () => {
      alive = false;
    };
  }, [query, errors]);

  const projects = useMemo(() => {
    if (!tree.tree) return [];
    const ix = index(tree.tree.nodes);
    return tree.tree.nodes
      .filter((n) => n.kind === "project")
      .map((n) => ({ value: n.id, label: n.name, detail: path(ix, n.id).map((p) => p.slug).join("/") }))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [tree.tree]);

  const go = (next: Partial<ScanParams>) => {
    const merged = { ...params, page: 1, ...next };
    const address = scanAddress(merged);
    router.replace(`/${locale}/admin/scans${address ? `?${address}` : ""}`, { scroll: false });
  };
  const toggleStatus = (s: ScanStatus) =>
    go({ status: params.status.includes(s) ? params.status.filter((x) => x !== s) : [...params.status, s] });
  const filtered = !!(params.project || params.kind || params.trigger || params.status.length);
  const current = page?.query === query ? page.data : null;
  const error = failure?.query === query ? failure.text : null;
  const codes = t.codes as unknown as CodeBook;

  return (
    <div className="grid gap-6">
      <PageHeader title={t.title} lead={t.lead} glyph={ScanSearch} />

      <section aria-label={t.filters.label} className="flex flex-wrap items-end gap-3 rounded-card border border-line bg-surface p-4 shadow-elev-1">
        <Field label={t.filters.project} className="min-w-0 flex-[2_1_16rem]">
          {(p) => (
            <Combobox
              {...p}
              value={params.project}
              options={projects}
              placeholder={t.filters.projectAny}
              onChange={(v) => {
                if (v !== params.project) go({ project: v });
              }}
            />
          )}
        </Field>
        <Field label={t.filters.kind} className="min-w-0 flex-[1_1_10rem]">
          {(p) => (
            <Select {...p} value={params.kind} onChange={(e) => go({ kind: e.target.value })}>
              <option value="">{t.filters.all}</option>
              <option value="collect">{t.kinds.collect}</option>
              <option value="index">{t.kinds.index}</option>
            </Select>
          )}
        </Field>
        <Field label={t.filters.trigger} className="min-w-0 flex-[1_1_10rem]">
          {(p) => (
            <Select {...p} value={params.trigger} onChange={(e) => go({ trigger: e.target.value })}>
              <option value="">{t.filters.all}</option>
              <option value="schedule">{t.triggers.schedule}</option>
              <option value="manual">{t.triggers.manual}</option>
            </Select>
          )}
        </Field>
        <fieldset className="grid min-w-0 gap-1.5">
          <legend className="mb-1.5 text-sm font-medium text-ink">{t.filters.status}</legend>
          <div className="flex flex-wrap gap-1.5">
            {SCAN_STATUSES.map((s) => {
              const on = params.status.includes(s);
              const Icon = icons[s];
              return (
                <button
                  key={s}
                  type="button"
                  aria-pressed={on}
                  onClick={() => toggleStatus(s)}
                  className={cn(
                    "motion-control inline-flex h-9 items-center gap-1.5 rounded-full border px-3 text-sm",
                    on ? "border-ink-2 bg-surface-3 text-ink" : "border-line text-muted hover:border-line-strong hover:text-ink",
                  )}
                >
                  <Icon aria-hidden="true" className="size-4" />
                  {t.statuses[s]}
                </button>
              );
            })}
          </div>
        </fieldset>
        {filtered && (
          <Button variant="ghost" onClick={() => router.replace(`/${locale}/admin/scans`, { scroll: false })}>
            {t.filters.reset}
          </Button>
        )}
      </section>

      {error && <Message note={{ kind: "error", text: error }} />}
      {!current && !error && <SkeletonTable rows={6} label={common.loading} />}
      {current && current.items.length === 0 && (
        <EmptyState icon={ScanSearch} title={filtered ? t.emptyFiltered : t.empty} text={filtered ? undefined : t.emptyText} />
      )}
      {current && current.items.length > 0 && (
        <>
          <Table>
            <thead>
              <tr>
                <Th className="w-10">
                  <span className="sr-only">{t.details.expand}</span>
                </Th>
                <Th>{t.columns.status}</Th>
                <Th>{t.columns.project}</Th>
                <Th className="hidden md:table-cell">{t.columns.kind}</Th>
                <Th>{t.columns.started}</Th>
                <Th numeric className="hidden sm:table-cell">
                  {t.columns.duration}
                </Th>
                <Th className="hidden lg:table-cell">{t.columns.summary}</Th>
              </tr>
            </thead>
            <tbody>
              {current.items.map((s) => {
                const expanded = open === s.id;
                const Icon = icons[s.status];
                const first = scanCodes(s)[0];
                const totals = scanTotals(s);
                return (
                  <Fragment key={s.id}>
                    <Tr>
                      <Td>
                        <button
                          type="button"
                          aria-expanded={expanded}
                          aria-controls={`scan-${s.id}`}
                          aria-label={expanded ? t.details.collapse : t.details.expand}
                          onClick={() => setOpen(expanded ? null : s.id)}
                          className="motion-control grid size-7 place-items-center rounded-md text-muted hover:bg-surface-3 hover:text-ink"
                        >
                          <ChevronDown aria-hidden="true" className={cn("motion-control size-4", expanded && "rotate-180")} />
                        </button>
                      </Td>
                      <Td>
                        <Badge tone={tones[s.status]}>
                          <Icon aria-hidden="true" className="size-3.5" />
                          {t.statuses[s.status]}
                        </Badge>
                        {first && <p className="mt-1 max-w-[18rem] truncate text-xs text-muted">{scanCodeText(codes, first, s.source).title}</p>}
                      </Td>
                      <Td>
                        <span className="flex items-center gap-1.5">
                          <Link href={catalogHref(locale, s.project.id)} className="font-medium text-ink hover:underline">
                            {s.project.name}
                          </Link>
                          <Link
                            href={catalogHref(locale, s.project.id, "docs")}
                            className="text-muted hover:text-signal"
                            aria-label={t.columns.docs}
                            title={t.columns.docs}
                          >
                            <BookOpen aria-hidden="true" className="size-3.5" />
                          </Link>
                        </span>
                        <span className="block font-mono text-xs text-muted">{s.project.path}</span>
                      </Td>
                      <Td className="hidden md:table-cell">
                        <span className="block text-ink-2">{t.kinds[s.kind]}</span>
                        <span className="block text-xs text-muted">{t.triggers[s.trigger]}</span>
                      </Td>
                      <Td>
                        <time dateTime={s.last_started_at} title={when(s.last_started_at, locale, "")} className="whitespace-nowrap">
                          {ago(s.last_started_at, locale, "")}
                        </time>
                        {s.repeats > 0 && (
                          <span
                            className="block text-xs text-muted"
                            title={seriesText(t.summary.series, s, (iso) => when(iso, locale, "")) ?? undefined}
                          >
                            {seriesText(t.summary.series, s, (iso) => ago(iso, locale, ""))}
                          </span>
                        )}
                      </Td>
                      <Td numeric className="hidden sm:table-cell">
                        {s.duration_ms === null ? "—" : formatDuration(s.duration_ms, locale)}
                      </Td>
                      <Td className="hidden text-sm text-ink-2 lg:table-cell">
                        {s.kind === "collect"
                          ? format(t.summary.collect, { branches: totals.branches, files: totals.files }) +
                            (totals.skipped ? ` · ${format(t.summary.skipped, { skipped: totals.skipped })}` : "")
                          : format(t.summary.index, { chunks: s.index?.embedded_chunks ?? 0, documents: s.index?.documents ?? 0 })}
                      </Td>
                    </Tr>
                    {expanded && (
                      <tr id={`scan-${s.id}`} className="bg-surface-2/60">
                        <td colSpan={7} className="px-4 py-4">
                          <ScanDetails scan={s} labels={t} codes={codes} />
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </Table>
          <Pager
            offset={(params.page - 1) * SCAN_PAGE}
            shown={current.items.length}
            total={current.total}
            size={SCAN_PAGE}
            labels={t.pager}
            onChange={(offset) => go({ ...params, page: Math.floor(offset / SCAN_PAGE) + 1 })}
          />
        </>
      )}
    </div>
  );
}

function ScanDetails({ scan, labels: t, codes }: { scan: Scan; labels: Labels; codes: CodeBook }) {
  const list = scanCodes(scan);
  return (
    <div className="grid gap-4 animate-rise-in">
      {list.map((code) => {
        const text = scanCodeText(codes, code, scan.source);
        const danger = scan.error?.code === code || scan.branches.some((b) => b.error === code);
        return (
          <div
            key={code}
            className={cn(
              "rounded-lg border px-4 py-3",
              danger ? "border-danger/40 bg-danger-soft/60" : "border-amber/40 bg-amber-soft/60",
            )}
          >
            <p className={cn("flex items-center gap-2 font-medium", danger ? "text-danger" : "text-amber")}>
              {danger ? <XCircle aria-hidden="true" className="size-4" /> : <TriangleAlert aria-hidden="true" className="size-4" />}
              {text.title}
              <code className="text-xs font-normal opacity-80">{code}</code>
            </p>
            {text.text && <p className="mt-1 text-sm text-ink-2">{text.text}</p>}
            {text.hint && (
              <p className="mt-1.5 text-sm text-ink">
                <span className="font-medium">{t.details.whatToDo}: </span>
                {text.hint}
              </p>
            )}
          </div>
        );
      })}

      <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-1.5 text-sm">
        <dt className="text-muted">{t.details.source}</dt>
        <dd className="text-ink">{scan.source ? t.sources[scan.source] : t.sources.none}</dd>
        {scan.index && (
          <>
            <dt className="text-muted">{t.details.engine}</dt>
            <dd className="text-ink">{scan.index.engine}</dd>
            <dt className="text-muted">{t.details.model}</dt>
            <dd className="font-mono text-ink">{scan.index.model || t.details.none}</dd>
            <dt className="text-muted">{t.details.embeddedFiles}</dt>
            <dd className="font-mono text-ink tabular-nums">{scan.index.embedded_files}</dd>
            <dt className="text-muted">{t.details.embeddedChunks}</dt>
            <dd className="font-mono text-ink tabular-nums">{scan.index.embedded_chunks}</dd>
            <dt className="text-muted">{t.details.documents}</dt>
            <dd className="font-mono text-ink tabular-nums">{scan.index.documents}</dd>
          </>
        )}
      </dl>

      {scan.branches.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.details.branch}</Th>
              <Th>{t.details.commit}</Th>
              <Th>{t.details.result}</Th>
              <Th numeric>{t.details.files}</Th>
              <Th>{t.details.skipped}</Th>
            </tr>
          </thead>
          <tbody>
            {scan.branches.map((b) => (
              <Tr key={b.name}>
                <Td className="font-mono text-xs">{b.name}</Td>
                <Td className="font-mono text-xs text-muted" title={b.commit}>
                  {b.commit ? shortCommit(b.commit).short : t.details.none}
                  {b.working_tree && (
                    <Badge tone="amber" className="ml-1.5 font-sans">
                      {t.details.workingTree}
                    </Badge>
                  )}
                </Td>
                <Td>
                  <Badge tone={b.result === "failed" ? "danger" : b.result === "collected" ? "signal" : "neutral"}>{t.results[b.result]}</Badge>
                  {b.truncated && (
                    <Badge tone="amber" className="ml-1.5">
                      {t.details.truncated}
                    </Badge>
                  )}
                </Td>
                <Td numeric>{b.files}</Td>
                <Td className="text-xs text-ink-2">
                  {Object.entries(b.skipped).length === 0
                    ? t.details.none
                    : Object.entries(b.skipped)
                        .map(([reason, n]) => `${t.skipReasons[reason as keyof Labels["skipReasons"]] ?? reason}: ${n}`)
                        .join(", ")}
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}

      {scan.error?.detail && (
        <details className="rounded-lg border border-line bg-surface px-4 py-2 text-sm">
          <summary className="cursor-pointer text-muted">{t.details.technical}</summary>
          <pre className="mt-2 font-mono text-xs break-all whitespace-pre-wrap text-ink-2">{scan.error.detail}</pre>
        </details>
      )}
    </div>
  );
}
