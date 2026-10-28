"use client";

import { ChevronRight, RotateCw } from "lucide-react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { errorCode } from "@/lib/api";
import { cn } from "@/lib/cn";
import { TREE_PAGE, buildRows, openQuery, parseOpen, toggleOpen, type Branch, type FlatRow, type TreeItem } from "@/lib/treeTable";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Table, Td, Th, Tr } from "../ui/Table";

export type TreeColumn<R> = { key: string; title: string; className?: string; numeric?: boolean; cell: (row: R, dim: boolean) => ReactNode };

export type TreePage<R> = { items: R[]; total?: number; next: string | null };

export type TreeGridLabels = {
  name: string;
  loading: string;
  more: string;
  retry: string;
  expand: string;
  collapse: string;
  label_aria: string;
  emptyBranch?: string;
};

type Props<R extends TreeItem> = {
  root: string;
  sourceKey: string;
  load: (parent: string, cursor: string | null, size: number) => Promise<TreePage<R>>;
  preset?: Map<string, Branch<R>> | null;
  name: (row: R, dim: boolean) => ReactNode;
  rowName: (row: R) => string;
  href: (row: R) => string;
  dim?: (row: R) => boolean;
  columns: TreeColumn<R>[];
  labels: TreeGridLabels;
  refresh?: number;
  pollMs?: (rows: R[]) => number | null;
  empty: ReactNode;
};

const empty = { rows: [], total: 0, loading: false, error: null, next: null };

export function PlaceTreeGrid<R extends TreeItem>({ root, sourceKey, load, preset, name, rowName, href, dim, columns, labels: t, refresh, pollMs, empty: emptyContent }: Props<R>) {
  const { errors } = useUiText();
  const params = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();
  const openParam = params.get("open");
  const openIds = useMemo(() => parseOpen(openParam), [openParam]);
  const filtered = !!preset;

  const [byParent, setByParent] = useState<Map<string, Branch<R>>>(() => new Map());
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());
  const [focus, setFocus] = useState(0);
  const rowRefs = useRef<(HTMLTableRowElement | null)[]>([]);
  const moved = useRef(false);
  const requested = useRef<Set<string>>(new Set());
  const loader = useRef(load);
  loader.current = load;

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const loadBranch = useCallback(
    async (id: string, cursor: string | null, size = TREE_PAGE) => {
      const put = (key: string, change: (b: Branch<R>) => Branch<R>) =>
        setByParent((m) => {
          const n = new Map(m);
          n.set(key, change(n.get(key) ?? empty));
          return n;
        });
      requested.current.add(id);
      put(id, (b) => ({ ...b, loading: true, error: null }));
      try {
        const page = await loader.current(id, cursor, size);
        put(id, (b) => {
          const rows = cursor === null ? page.items : [...b.rows, ...page.items];
          return { rows, total: page.total ?? rows.length, loading: false, error: null, next: page.next };
        });
      } catch (e) {
        put(id, (b) => ({ ...b, loading: false, error: fail(e) }));
      }
    },
    [fail],
  );

  useEffect(() => {
    requested.current = new Set();
    setByParent(new Map());
    setCollapsed(new Set());
    if (!filtered) void loadBranch(root, null);
  }, [filtered, loadBranch, root, sourceKey]);

  const tree = preset ?? byParent;
  const open = useMemo(
    () => (filtered ? new Set([...tree.keys()].filter((id) => !collapsed.has(id))) : new Set(openIds)),
    [filtered, tree, collapsed, openIds],
  );
  const rows = useMemo(() => buildRows(tree, open, root, !filtered, !!t.emptyBranch), [tree, open, root, filtered, t.emptyBranch]);

  useEffect(() => {
    if (filtered) return;
    for (const r of rows) {
      if (r.type === "node" && r.expanded && !requested.current.has(r.row.id)) void loadBranch(r.row.id, null);
    }
  }, [rows, filtered, loadBranch]);

  const reloadLoaded = useCallback(
    (ids: Iterable<string>) => {
      for (const id of ids) {
        const b = byParent.get(id);
        void loadBranch(id, null, Math.min(200, Math.max(TREE_PAGE, b?.rows.length ?? 0)));
      }
    },
    [byParent, loadBranch],
  );

  const shown = useMemo(() => rows.flatMap((r) => (r.type === "node" ? [r.row] : [])), [rows]);
  const every = pollMs?.(shown) ?? null;
  useEffect(() => {
    if (filtered || every === null) return;
    const timer = setTimeout(() => {
      if (document.visibilityState === "hidden") return;
      reloadLoaded([...byParent].filter(([, b]) => !b.loading && !b.error && b.rows.length > 0).map(([id]) => id));
    }, every);
    return () => clearTimeout(timer);
  }, [every, byParent, filtered, reloadLoaded]);

  const seenRefresh = useRef(refresh);
  useEffect(() => {
    if (seenRefresh.current === refresh) return;
    seenRefresh.current = refresh;
    if (!filtered) reloadLoaded(new Set([root, ...requested.current]));
  }, [refresh, filtered, reloadLoaded, root]);

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
    const sp = new URLSearchParams(params.toString());
    const next = openQuery(toggleOpen(openIds, id));
    if (next) sp.set("open", next);
    else sp.delete("open");
    router.replace(`${pathname}?${sp.toString()}`, { scroll: false });
  };

  const more = (parent: string) => {
    const b = tree.get(parent);
    void loadBranch(parent, b?.next ?? String(b?.rows.length ?? 0));
  };

  const go = (i: number) => {
    moved.current = true;
    setFocus(Math.max(0, Math.min(rows.length - 1, i)));
  };

  const onKey = (e: KeyboardEvent<HTMLTableRowElement>, i: number, r: FlatRow<R>) => {
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
        if (node) router.push(href(node.row));
        else if (r.type === "more") more(r.parent);
        else if (r.type === "error") void loadBranch(r.parent, null);
        break;
      default:
        return;
    }
    e.preventDefault();
  };

  const indent = (level: number) => ({ paddingInlineStart: `${(level - 1) * 1.25}rem` });
  const rootBranch = tree.get(root);
  const loadingRoot = !filtered && (!rootBranch || (rootBranch.loading && rootBranch.rows.length === 0));
  const span = columns.length + 1;

  if (!loadingRoot && rows.length === 0) {
    return rootBranch?.error ? <p className="text-sm text-danger">{rootBranch.error}</p> : <>{emptyContent}</>;
  }

  return (
    <Table role="treegrid" aria-label={t.label_aria} aria-busy={loadingRoot}>
      <thead>
        <tr>
          <Th>{t.name}</Th>
          {columns.map((c) => (
            <Th key={c.key} numeric={c.numeric} className={c.className}>
              {c.title}
            </Th>
          ))}
        </tr>
      </thead>
      <tbody>
        {loadingRoot && (
          <tr>
            <Td colSpan={span} className="text-muted">
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
                <Td colSpan={span}>
                  <span className="flex items-center gap-2" style={indent(r.level)}>
                    <span className="size-6 shrink-0" />
                    {r.type === "loading" && <span className="text-muted">{t.loading}</span>}
                    {r.type === "empty" && <span className="text-muted">{t.emptyBranch}</span>}
                    {r.type === "more" && (
                      <Button size="sm" variant="ghost" tabIndex={-1} onClick={() => more(r.parent)}>
                        {t.more}
                      </Button>
                    )}
                    {r.type === "error" && (
                      <>
                        <span className="text-danger">{r.error}</span>
                        <Button size="sm" variant="ghost" tabIndex={-1} onClick={() => void loadBranch(r.parent, null)}>
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
          const faded = filtered && !!dim?.(row);
          return (
            <Tr key={row.id} {...common} aria-expanded={r.expandable ? r.expanded : undefined} aria-setsize={r.setsize} aria-posinset={r.posinset}>
              <Td>
                <span className="flex min-w-0 items-center gap-2" style={indent(r.level)}>
                  {r.expandable ? (
                    <button
                      type="button"
                      tabIndex={-1}
                      onClick={() => toggle(row.id)}
                      aria-label={format(r.expanded ? t.collapse : t.expand, { name: rowName(row) })}
                      className="motion-control grid size-6 shrink-0 place-items-center rounded text-muted hover:bg-surface-3 hover:text-ink"
                    >
                      <ChevronRight aria-hidden="true" className={cn("size-4 transition-transform", r.expanded && "rotate-90")} />
                    </button>
                  ) : (
                    <span className="size-6 shrink-0" />
                  )}
                  {name(row, faded)}
                </span>
              </Td>
              {columns.map((c) => (
                <Td key={c.key} numeric={c.numeric} className={c.className}>
                  {c.cell(row, faded)}
                </Td>
              ))}
            </Tr>
          );
        })}
      </tbody>
    </Table>
  );
}
