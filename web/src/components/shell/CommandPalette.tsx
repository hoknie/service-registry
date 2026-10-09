"use client";

import { CornerDownLeft, FileSearch, Search } from "lucide-react";
import { useRouter } from "next/navigation";
import { Dialog as D } from "radix-ui";
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode, type CatalogNode } from "@/lib/api";
import { useCatalogTree } from "@/lib/catalogTree";
import { cn } from "@/lib/cn";

import { KindIcon } from "../catalog/KindIcon";
import { useUiText } from "../UiText";
import { Kbd } from "../ui/Kbd";
import type { NavItem } from "./nav";

type Labels = Messages["palette"];
type Option = { id: string; href: string; label: string; detail?: string; group: "pages" | "nodes"; node?: CatalogNode; page?: NavItem };

const LIMIT = 50;

export function CommandPalette({
  open,
  onOpenChange,
  locale,
  pages,
  labels,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  locale: Locale;
  pages: NavItem[];
  labels: Labels;
}) {
  const { errors } = useUiText();
  const router = useRouter();
  const listId = useId();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const catalog = useCatalogTree(open);
  const nodes = catalog.tree?.nodes ?? null;
  const error = catalog.error ? errorText(errors, errorCode(catalog.error)) : null;
  const list = useRef<HTMLUListElement>(null);

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && !e.altKey && e.key.toLowerCase() === "k") {
        e.preventDefault();
        onOpenChange(!open);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onOpenChange]);

  const paths = useMemo(() => {
    const byId = new Map((nodes ?? []).map((n) => [n.id, n]));
    const path = (n: CatalogNode): string => {
      const names: string[] = [];
      let parent = n.parent_id ? byId.get(n.parent_id) : undefined;
      while (parent) {
        names.unshift(parent.name);
        parent = parent.parent_id ? byId.get(parent.parent_id) : undefined;
      }
      return names.join(" / ");
    };
    return new Map((nodes ?? []).map((n) => [n.id, path(n)]));
  }, [nodes]);

  const options = useMemo<Option[]>(() => {
    const q = query.trim().toLowerCase();
    const match = (...texts: (string | undefined)[]) => !q || texts.some((t) => t?.toLowerCase().includes(q));
    const pageOptions: Option[] = pages
      .filter((p) => match(p.label))
      .map((p) => ({ id: `page-${p.key}`, href: p.href, label: p.label, group: "pages", page: p }));
    const nodeOptions: Option[] = (nodes ?? [])
      .filter((n) => match(n.name, n.slug))
      .slice(0, LIMIT)
      .map((n) => ({
        id: `node-${n.id}`,
        href: `/${locale}/catalog?node=${n.id}`,
        label: n.name,
        detail: paths.get(n.id),
        group: "nodes",
        node: n,
      }));
    const searchOptions: Option[] = q
      ? [
          {
            id: "search-docs",
            href: `/${locale}/search?q=${encodeURIComponent(query.trim())}`,
            label: format(labels.searchDocs, { q: query.trim() }),
            group: "pages",
            page: { key: "search-docs", href: `/${locale}/search`, label: labels.searchDocs, icon: FileSearch },
          },
        ]
      : [];
    return [...pageOptions, ...nodeOptions, ...searchOptions];
  }, [query, pages, nodes, paths, locale, labels.searchDocs]);

  const current = Math.min(active, Math.max(0, options.length - 1));

  useEffect(() => {
    list.current?.querySelector(`[data-index="${current}"]`)?.scrollIntoView({ block: "nearest" });
  }, [current]);

  const go = (option: Option | undefined) => {
    if (!option) return;
    onOpenChange(false);
    router.push(option.href);
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((current + 1) % Math.max(1, options.length));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((current - 1 + options.length) % Math.max(1, options.length));
    } else if (e.key === "Enter") {
      e.preventDefault();
      go(options[current]);
    }
  };

  const groupTitle = { pages: labels.pages, nodes: labels.nodes };

  return (
    <D.Root
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setQuery("");
          setActive(0);
        }
        onOpenChange(next);
      }}
    >
      <D.Portal>
        <D.Overlay className="fixed inset-0 z-50 animate-fade-in bg-overlay backdrop-blur-[2px]" />
        <D.Content
          className="fixed top-[12vh] left-1/2 z-50 w-[calc(100vw-2rem)] max-w-xl -translate-x-1/2 animate-pop-in overflow-hidden rounded-xl border border-line bg-surface shadow-pop focus:outline-none"
        >
          <D.Title className="sr-only">{labels.title}</D.Title>
          <D.Description className="sr-only">{labels.hint}</D.Description>
          <div className="flex items-center gap-3 border-b border-line px-4">
            <Search aria-hidden="true" className="size-4 shrink-0 text-muted" />
            <input
              role="combobox"
              aria-expanded="true"
              aria-controls={listId}
              aria-autocomplete="list"
              aria-activedescendant={options[current] ? `${listId}-${current}` : undefined}
              aria-label={labels.title}
              autoFocus
              value={query}
              placeholder={labels.placeholder}
              onChange={(e) => {
                setQuery(e.target.value);
                setActive(0);
              }}
              onKeyDown={onKeyDown}
              className="h-12 w-full bg-transparent text-base text-ink placeholder:text-muted focus-visible:outline-none"
            />
            <Kbd>Esc</Kbd>
          </div>
          <ul ref={list} id={listId} role="listbox" aria-label={labels.title} className="max-h-[min(60vh,26rem)] overflow-y-auto p-2">
            {options.map((o, i) => (
              <li key={o.id} role="presentation">
                {(i === 0 || options[i - 1]?.group !== o.group) && (
                  <p role="presentation" className="px-2.5 pt-2 pb-1 text-xs text-muted">
                    {groupTitle[o.group]}
                  </p>
                )}
                <div
                  id={`${listId}-${i}`}
                  role="option"
                  aria-selected={i === current}
                  data-index={i}
                  onMouseMove={() => setActive(i)}
                  onClick={() => go(o)}
                  className={cn(
                    "flex cursor-pointer items-center gap-3 rounded-md px-2.5 py-2 text-sm",
                    i === current ? "bg-surface-3 text-ink" : "text-ink-2",
                  )}
                >
                  {o.node ? (
                    <KindIcon kind={o.node.kind} />
                  ) : (
                    o.page && <o.page.icon aria-hidden="true" className="size-4 shrink-0 text-muted" />
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate font-medium text-ink">{o.label}</span>
                    {o.detail && <span className="block truncate text-xs text-muted">{o.detail}</span>}
                  </span>
                  {i === current && <CornerDownLeft aria-hidden="true" className="size-4 text-muted" />}
                </div>
              </li>
            ))}
          </ul>
          {(options.length === 0 || (!nodes && !error) || error) && (
            <p role="status" className="px-5 pb-5 text-sm text-muted">
              {error ?? (!nodes ? labels.loading : labels.empty)}
            </p>
          )}
          <p className="border-t border-line bg-surface-2 px-4 py-2 text-xs text-muted">{labels.hint}</p>
        </D.Content>
      </D.Portal>
    </D.Root>
  );
}
