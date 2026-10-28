"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { BUSY_MS, IDLE_MS, busy, isSummary } from "@/lib/activity";
import { apiGet, errorCode, type CatalogTable, type CatalogTableRow } from "@/lib/api";
import { useCatalogVersion } from "@/lib/catalogEvents";
import { cn } from "@/lib/cn";
import { badgeNames } from "@/lib/owners";
import { filtersQuery, groupByParent, hasFilters, highlight, parseFilters, type Branch } from "@/lib/treeTable";

import { useUiText } from "../UiText";
import { Badge, LabelChip } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { LinkIconRow } from "../links/LinkIcons";
import { OwnerFilter } from "../owners/OwnerFilter";
import { PlaceTreeGrid, type TreeColumn } from "../places/PlaceTreeGrid";
import { ProcessBadges, SummaryBadges } from "./ActivityBadges";
import { LabelFilter } from "./LabelFilter";
import { KindIcon } from "./KindIcon";
import { catalogHref, type CatalogLabels } from "./shared";

type Props = { rootId: string | null; locale: Locale; labels: CatalogLabels };

const DEBOUNCE_MS = 300;

export function TreeTable({ rootId, locale, labels }: Props) {
  const t = labels.tree;
  const { errors } = useUiText();
  const params = useSearchParams();
  const router = useRouter();
  const root = rootId ?? "";
  const filters = parseFilters(params);
  const filtered = hasFilters(filters);
  const query = filtersQuery(filters);

  const [preset, setPreset] = useState<Map<string, Branch> | null>(null);
  const [truncated, setTruncated] = useState(false);
  const [draft, setDraft] = useState({ q: filters.q, label: filters.label });

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const parentQuery = (id: string) => (id ? `parent=${id}&` : "");

  const navigate = useCallback(
    (next: Record<string, string>) => {
      const sp = new URLSearchParams(params.toString());
      for (const [k, v] of Object.entries(next)) {
        if (v) sp.set(k, v);
        else sp.delete(k);
      }
      router.replace(`/${locale}/catalog?${sp.toString()}`, { scroll: false });
    },
    [params, router, locale],
  );

  const load = useCallback(async (parent: string, cursor: string | null, size: number) => {
    const offset = Number(cursor ?? 0);
    const page = await apiGet<CatalogTable>(`/v1/catalog/table?${parentQuery(parent)}limit=${size}&offset=${offset}`);
    const end = offset + page.items.length;
    return { items: page.items, total: page.total, next: end < page.total ? String(end) : null };
  }, []);

  const loadFiltered = useCallback(async () => {
    try {
      const page = await apiGet<CatalogTable>(`/v1/catalog/table?${parentQuery(root)}${query}`);
      setPreset(groupByParent(page.items, root));
      setTruncated(page.truncated);
    } catch (e) {
      setPreset(new Map([[root, { rows: [], total: 0, loading: false, error: fail(e) }]]));
      setTruncated(false);
    }
  }, [root, query, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (filtered) void loadFiltered();
    else setPreset(null);
  }, [filtered, loadFiltered]);

  const presetBusy = !!preset && [...preset.values()].some((b) => b.rows.some((r) => busy(r.activity)));
  useEffect(() => {
    if (!filtered) return;
    const timer = setTimeout(() => {
      if (document.visibilityState !== "hidden") void loadFiltered();
    }, presetBusy ? BUSY_MS : IDLE_MS);
    return () => clearTimeout(timer);
  }, [filtered, presetBusy, preset, loadFiltered]);

  const version = useCatalogVersion();
  const seenVersion = useRef(version);
  useEffect(() => {
    if (seenVersion.current === version || !filtered) return;
    seenVersion.current = version;
    void loadFiltered();
  }, [version, filtered, loadFiltered]);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (draft.q !== filters.q || draft.label !== filters.label) navigate({ q: draft.q.trim(), label: draft.label.trim() });
    }, DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [draft, filters.q, filters.label, navigate]);

  const reset = () => {
    setDraft({ q: "", label: "" });
    navigate({ q: "", kind: "", label: "", activity: "", owner: "" });
  };

  const activityCell = (row: CatalogTableRow) =>
    isSummary(row.activity) ? (
      <SummaryBadges summary={row.activity} labels={labels.activity} />
    ) : (
      <ProcessBadges processes={row.activity} labels={labels.activity} locale={locale} compact />
    );

  const columns: TreeColumn<CatalogTableRow>[] = [
    { key: "slug", title: t.slug, className: "hidden sm:table-cell", cell: (row, dim) => <code className={cn("text-xs", dim ? "text-muted" : "text-ink-2")}>{row.slug}</code> },
    {
      key: "labels",
      title: t.labels,
      className: "hidden lg:table-cell",
      cell: (row) => {
        const labelEntries = Object.entries(row.labels ?? {});
        return (
          <span className="flex flex-wrap gap-1">
            {labelEntries.slice(0, 3).map(([k, v]) => (
              <LabelChip key={k} name={k} value={v} />
            ))}
            {labelEntries.length > 3 && <Badge tone="outline">+{labelEntries.length - 3}</Badge>}
          </span>
        );
      },
    },
    { key: "owners", title: t.owners, className: "hidden @min-[640px]:table-cell", cell: (row, dim) => <OwnerNames owners={row.owners ?? []} dim={dim} /> },
    {
      key: "activity",
      title: t.activity,
      cell: (row) => (
        <span className="grid gap-1.5">
          {activityCell(row)}
          {row.links?.length > 0 && <LinkIconRow links={row.links} locale={locale} labels={labels.linkIcons} max={6} />}
        </span>
      ),
    },
    { key: "children", title: t.children, numeric: true, className: "hidden text-muted sm:table-cell", cell: (row) => (row.kind === "project" ? "" : row.children) },
  ];

  return (
    <section className="@container grid gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Input
          type="search"
          value={draft.q}
          onChange={(e) => setDraft((d) => ({ ...d, q: e.target.value }))}
          placeholder={t.search}
          aria-label={t.search}
          maxLength={100}
          className="w-56 max-w-full"
        />
        <Select aria-label={t.kind} value={filters.kind} onChange={(e) => navigate({ kind: e.target.value })} className="w-40">
          <option value="">{t.anyKind}</option>
          <option value="organization">{labels.kinds.organization}</option>
          <option value="folder">{labels.kinds.folder}</option>
          <option value="project">{labels.kinds.project}</option>
        </Select>
        <LabelFilter
          value={draft.label}
          onChange={(label) => setDraft((d) => ({ ...d, label }))}
          placeholder={t.label}
          label={t.label}
          hint={t.labelHint}
          className="w-44 max-w-full"
        />
        <Select aria-label={t.state} value={filters.activity} onChange={(e) => navigate({ activity: e.target.value })} className="w-40">
          <option value="">{t.anyState}</option>
          <option value="running">{t.running}</option>
          <option value="queued">{t.queued}</option>
          <option value="failed">{t.failed}</option>
        </Select>
        <OwnerFilter value={filters.owner} onChange={(owner) => navigate({ owner })} labels={t} className="w-44 max-w-full" />
        {filtered && (
          <Button variant="ghost" onClick={reset}>
            {t.reset}
          </Button>
        )}
      </div>

      {filtered && truncated && <p className="text-sm text-amber">{t.truncated}</p>}

      {filtered && !preset ? (
        <p className="text-sm text-muted">{t.loading}</p>
      ) : (
        <PlaceTreeGrid
          root={root}
          sourceKey="catalog"
          load={load}
          preset={filtered ? preset : null}
          labels={t}
          refresh={version}
          pollMs={(rows) => (rows.some((r) => busy(r.activity)) ? BUSY_MS : IDLE_MS)}
          href={(row) => catalogHref(locale, row.id)}
          rowName={(row) => row.name}
          dim={(row) => !row.match}
          columns={columns}
          name={(row, dim) => {
            const parts = filtered ? highlight(row.name, filters.q) : [{ text: row.name, hit: false }];
            return (
              <>
                <KindIcon kind={row.kind} />
                <Link href={catalogHref(locale, row.id)} tabIndex={-1} className={cn("min-w-0 truncate font-medium hover:underline", dim ? "text-muted" : "text-ink")}>
                  {parts.map((p, k) =>
                    p.hit ? (
                      <mark key={k} className="rounded-sm bg-amber-soft px-0.5 text-ink">
                        {p.text}
                      </mark>
                    ) : (
                      <span key={k}>{p.text}</span>
                    ),
                  )}
                </Link>
                {row.access === "navigate" && <Badge tone="outline">{labels.navigateOnly}</Badge>}
                {row.visibility === "open" && <Badge tone="cobalt">{labels.visibility.badge}</Badge>}
              </>
            );
          }}
          empty={
            <div className="flex flex-wrap items-center gap-3 text-sm text-muted">
              {filtered ? t.noMatch : labels.children.empty}
              {filtered && (
                <Button size="sm" onClick={reset}>
                  {t.reset}
                </Button>
              )}
            </div>
          }
        />
      )}
    </section>
  );
}

function OwnerNames({ owners, dim }: { owners: CatalogTableRow["owners"]; dim: boolean }) {
  const { names, more } = badgeNames(owners);
  if (names.length === 0) return null;
  return (
    <span className={cn("flex flex-wrap items-center gap-1 text-sm", dim ? "text-muted" : "text-ink-2")}>
      <span className="truncate">{names.join(", ")}</span>
      {more > 0 && <Badge tone="outline">+{more}</Badge>}
    </span>
  );
}
