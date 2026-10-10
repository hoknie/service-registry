"use client";

import { ChevronRight, RotateCw } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { BUSY_MS, IDLE_MS, busy, isSummary } from "@/lib/activity";
import { apiGet, errorCode, type CatalogTable, type CatalogTableRow } from "@/lib/api";
import { cn } from "@/lib/cn";
import {
  TREE_PAGE,
  buildRows,
  filtersQuery,
  groupByParent,
  hasFilters,
  highlight,
  openQuery,
  parseFilters,
  parseOpen,
  toggleOpen,
  type Branch,
  type FlatRow,
} from "@/lib/treeTable";

import { useUiText } from "../UiText";
import { Badge, LabelChip } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Table, Td, Th, Tr } from "../ui/Table";
import { LinkIconRow } from "../links/LinkIcons";
import { ProcessBadges, SummaryBadges } from "./ActivityBadges";
import { LabelFilter } from "./LabelFilter";
import { KindIcon } from "./KindIcon";
import { catalogHref, type CatalogLabels } from "./shared";

type Props = { rootId: string | null; locale: Locale; labels: CatalogLabels };

const DEBOUNCE_MS = 300;
const empty: Branch = { rows: [], total: 0, loading: false, error: null };

export function TreeTable({ rootId, locale, labels }: Props) {
  const t = labels.tree;
  const { errors } = useUiText();
  const params = useSearchParams();
  const router = useRouter();
  const root = rootId ?? "";
  const filters = parseFilters(params);
  const filtered = hasFilters(filters);
  const query = filtersQuery(filters);
  const openParam = params.get("open");
  const openIds = useMemo(() => parseOpen(openParam), [openParam]);

  const [byParent, setByParent] = useState<Map<string, Branch>>(() => new Map());
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());
  const [truncated, setTruncated] = useState(false);
  const [draft, setDraft] = useState({ q: filters.q, label: filters.label });
  const [focus, setFocus] = useState(0);
  const rowRefs = useRef<(HTMLTableRowElement | null)[]>([]);
  const moved = useRef(false);
  const requested = useRef<Set<string>>(new Set());

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

  const put = (id: string, change: (b: Branch) => Branch) =>
    setByParent((m) => {
      const n = new Map(m);
      n.set(id, change(n.get(id) ?? empty));
      return n;
    });

  const loadBranch = useCallback(
    async (id: string, offset: number, limit = TREE_PAGE) => {
      requested.current.add(id);
      put(id, (b) => ({ ...b, loading: true, error: null }));
      try {
        const page = await apiGet<CatalogTable>(`/v1/catalog/table?${parentQuery(id)}limit=${limit}&offset=${offset}`);
        put(id, (b) => ({ rows: offset === 0 ? page.items : [...b.rows, ...page.items], total: page.total, loading: false, error: null }));
      } catch (e) {
        put(id, (b) => ({ ...b, loading: false, error: fail(e) }));
      }
    },
    [fail],
  );

  const loadFiltered = useCallback(async () => {
    try {
      const page = await apiGet<CatalogTable>(`/v1/catalog/table?${parentQuery(root)}${query}`);
      setByParent(groupByParent(page.items, root));
      setTruncated(page.truncated);
    } catch (e) {
      setByParent(new Map([[root, { ...empty, error: fail(e) }]]));
      setTruncated(false);
    }
  }, [root, query, fail]);

  useEffect(() => {
    requested.current = new Set();
    setByParent(new Map());
    setCollapsed(new Set());
    if (filtered) void loadFiltered();
    else void loadBranch(root, 0);
  }, [filtered, loadFiltered, loadBranch, root]);

  const open = useMemo(
    () => (filtered ? new Set([...byParent.keys()].filter((id) => !collapsed.has(id))) : new Set(openIds)),
    [filtered, byParent, collapsed, openIds],
  );
  const rows = useMemo(() => buildRows(byParent, open, root, !filtered), [byParent, open, root, filtered]);

  useEffect(() => {
    if (filtered) return;
    for (const r of rows) {
      if (r.type === "node" && r.expanded && !requested.current.has(r.row.id)) void loadBranch(r.row.id, 0);
    }
  }, [rows, filtered, loadBranch]);

  const anyBusy = rows.some((r) => r.type === "node" && busy(r.row.activity));
  useEffect(() => {
    const timer = setTimeout(() => {
      if (document.visibilityState === "hidden") return;
      if (filtered) {
        void loadFiltered();
        return;
      }
      for (const [id, b] of byParent) {
        if (!b.loading && !b.error && b.rows.length > 0) void loadBranch(id, 0, Math.min(200, Math.max(TREE_PAGE, b.rows.length)));
      }
    }, anyBusy ? BUSY_MS : IDLE_MS);
    return () => clearTimeout(timer);
  }, [anyBusy, byParent, filtered, loadFiltered, loadBranch]);

  useEffect(() => {
    const timer = setTimeout(() => {
      if (draft.q !== filters.q || draft.label !== filters.label) navigate({ q: draft.q.trim(), label: draft.label.trim() });
    }, DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [draft, filters.q, filters.label, navigate]);

  useEffect(() => {
    if (!moved.current) return;
    moved.current = false;
    rowRefs.current[focus]?.focus();
  }, [focus]);

  const toggle = (id: string) => {
    if (filtered) {
      setCollapsed((s) => {
        const n = new Set(s);
        if (n.has(id)) n.delete(id);
        else n.add(id);
        return n;
      });
      return;
    }
    navigate({ open: openQuery(toggleOpen(openIds, id)) });
  };

  const reset = () => {
    setDraft({ q: "", label: "" });
    navigate({ q: "", kind: "", label: "", activity: "" });
  };

  const go = (i: number) => {
    const next = Math.max(0, Math.min(rows.length - 1, i));
    moved.current = true;
    setFocus(next);
  };

  const onKey = (e: KeyboardEvent<HTMLTableRowElement>, i: number, r: FlatRow) => {
    const node = r.type === "node" ? r : null;
    switch (e.key) {
      case "ArrowDown":
        go(i + 1);
        break;
      case "ArrowUp":
        go(i - 1);
        break;
      case "Home":
        go(0);
        break;
      case "End":
        go(rows.length - 1);
        break;
      case "ArrowRight":
        if (node?.expandable && !node.expanded) toggle(node.row.id);
        else if (node?.expanded) go(i + 1);
        break;
      case "ArrowLeft":
        if (node?.expanded) toggle(node.row.id);
        else {
          for (let j = i - 1; j >= 0; j--) {
            if ((rows[j]?.level ?? 0) < r.level) {
              go(j);
              break;
            }
          }
        }
        break;
      case "Enter":
        if (node) router.push(catalogHref(locale, node.row.id));
        else if (r.type === "more") void loadBranch(r.parent, byParent.get(r.parent)?.rows.length ?? 0);
        else if (r.type === "error") void loadBranch(r.parent, 0);
        break;
      default:
        return;
    }
    e.preventDefault();
  };

  const indent = (level: number) => ({ paddingInlineStart: `${(level - 1) * 1.25}rem` });
  const rootBranch = byParent.get(root);
  const loadingRoot = !rootBranch || (rootBranch.loading && rootBranch.rows.length === 0);

  const activityCell = (row: CatalogTableRow) =>
    isSummary(row.activity) ? (
      <SummaryBadges summary={row.activity} labels={labels.activity} />
    ) : (
      <ProcessBadges processes={row.activity} labels={labels.activity} locale={locale} compact />
    );

  return (
    <section className="grid gap-3">
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
        {filtered && (
          <Button variant="ghost" onClick={reset}>
            {t.reset}
          </Button>
        )}
      </div>

      {filtered && truncated && <p className="text-sm text-amber">{t.truncated}</p>}

      {!loadingRoot && rows.length === 0 ? (
        rootBranch?.error ? (
          <p className="text-sm text-danger">{rootBranch.error}</p>
        ) : (
          <div className="flex flex-wrap items-center gap-3 text-sm text-muted">
            {filtered ? t.noMatch : labels.children.empty}
            {filtered && (
              <Button size="sm" onClick={reset}>
                {t.reset}
              </Button>
            )}
          </div>
        )
      ) : (
        <Table role="treegrid" aria-label={t.label_aria} aria-busy={loadingRoot}>
          <thead>
            <tr>
              <Th>{t.name}</Th>
              <Th className="hidden sm:table-cell">{t.slug}</Th>
              <Th className="hidden lg:table-cell">{t.labels}</Th>
              <Th>{t.activity}</Th>
              <Th numeric className="hidden sm:table-cell">
                {t.children}
              </Th>
            </tr>
          </thead>
          <tbody>
            {loadingRoot && (
              <tr>
                <Td colSpan={5} className="text-muted">
                  {t.loading}
                </Td>
              </tr>
            )}
            {rows.map((r, i) => {
              const common = {
                ref: (el: HTMLTableRowElement | null) => {
                  rowRefs.current[i] = el;
                },
                tabIndex: i === Math.min(focus, rows.length - 1) ? 0 : -1,
                onKeyDown: (e: KeyboardEvent<HTMLTableRowElement>) => onKey(e, i, r),
                onFocus: () => setFocus(i),
                "aria-level": r.level,
                className: "outline-none focus-visible:bg-surface-2 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
              };
              if (r.type !== "node") {
                return (
                  <Tr key={`${r.parent}:${r.type}`} {...common}>
                    <Td colSpan={5}>
                      <span className="flex items-center gap-2" style={indent(r.level)}>
                        <span className="size-6 shrink-0" />
                        {r.type === "loading" && <span className="text-muted">{t.loading}</span>}
                        {r.type === "more" && (
                          <Button size="sm" variant="ghost" tabIndex={-1} onClick={() => void loadBranch(r.parent, byParent.get(r.parent)?.rows.length ?? 0)}>
                            {t.more}
                          </Button>
                        )}
                        {r.type === "error" && (
                          <>
                            <span className="text-danger">{r.error}</span>
                            <Button size="sm" variant="ghost" tabIndex={-1} onClick={() => void loadBranch(r.parent, 0)}>
                              <RotateCw aria-hidden="true" />
                              {t.retry}
                            </Button>
                          </>
                        )}
                      </span>
                    </Td>
                  </Tr>
                );
              }
              const row = r.row;
              const dim = filtered && !row.match;
              const parts = filtered ? highlight(row.name, filters.q) : [{ text: row.name, hit: false }];
              const labelEntries = Object.entries(row.labels ?? {});
              return (
                <Tr
                  key={row.id}
                  {...common}
                  aria-expanded={r.expandable ? r.expanded : undefined}
                  aria-setsize={r.setsize}
                  aria-posinset={r.posinset}
                >
                  <Td>
                    <span className="flex min-w-0 items-center gap-2" style={indent(r.level)}>
                      {r.expandable ? (
                        <button
                          type="button"
                          tabIndex={-1}
                          onClick={() => toggle(row.id)}
                          aria-label={format(r.expanded ? t.collapse : t.expand, { name: row.name })}
                          className="motion-control grid size-6 shrink-0 place-items-center rounded text-muted hover:bg-surface-3 hover:text-ink"
                        >
                          <ChevronRight aria-hidden="true" className={cn("size-4 transition-transform", r.expanded && "rotate-90")} />
                        </button>
                      ) : (
                        <span className="size-6 shrink-0" />
                      )}
                      <KindIcon kind={row.kind} />
                      <Link
                        href={catalogHref(locale, row.id)}
                        tabIndex={-1}
                        className={cn("min-w-0 truncate font-medium hover:underline", dim ? "text-muted" : "text-ink")}
                      >
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
                    </span>
                  </Td>
                  <Td className="hidden sm:table-cell">
                    <code className={cn("text-xs", dim ? "text-muted" : "text-ink-2")}>{row.slug}</code>
                  </Td>
                  <Td className="hidden lg:table-cell">
                    <span className="flex flex-wrap gap-1">
                      {labelEntries.slice(0, 3).map(([k, v]) => (
                        <LabelChip key={k} name={k} value={v} />
                      ))}
                      {labelEntries.length > 3 && <Badge tone="outline">+{labelEntries.length - 3}</Badge>}
                    </span>
                  </Td>
                  <Td>
                    <span className="grid gap-1.5">
                      {activityCell(row)}
                      {row.links?.length > 0 && <LinkIconRow links={row.links} locale={locale} labels={labels.linkIcons} max={6} />}
                    </span>
                  </Td>
                  <Td numeric className="hidden text-muted sm:table-cell">
                    {row.kind === "project" ? "" : row.children}
                  </Td>
                </Tr>
              );
            })}
          </tbody>
        </Table>
      )}
    </section>
  );
}
