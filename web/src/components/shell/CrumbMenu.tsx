"use client";

import { Check, ChevronDown, CornerLeftUp } from "lucide-react";
import { useRouter } from "next/navigation";
import { Popover as P } from "radix-ui";
import { useId, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { childrenOf, index, siblings } from "@/lib/catalogNav";
import { useCatalogTree } from "@/lib/catalogTree";
import { cn } from "@/lib/cn";

import { KindIcon } from "../catalog/KindIcon";
import { catalogHref } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Spinner } from "../ui/Spinner";

type Labels = Messages["header"]["navigator"];
export type MenuEntry = { key: string; id: string; name: string; slug?: string; href: string; group: "up" | "near" | "inside"; icon: ReactNode };

const FILTER_FROM = 11;
const SHOWN = 200;

export function CrumbMenu({ node, name, last, locale, labels }: { node: string; name: string; last: boolean; locale: Locale; labels: Labels }) {
  const [open, setOpen] = useState(false);
  const { tree, error, loading } = useCatalogTree(open);

  const all = useMemo<MenuEntry[]>(() => {
    if (!tree) return [];
    const ix = index(tree.nodes);
    const entry = (group: MenuEntry["group"], n: (typeof tree.nodes)[number]): MenuEntry => ({
      key: `${group}-${n.id}`,
      id: n.id,
      name: n.name,
      slug: n.slug,
      href: catalogHref(locale, n.id),
      group,
      icon: <KindIcon kind={n.kind} />,
    });
    if (node === "root") return childrenOf(ix, null).map((n) => entry("inside", n));
    const self = ix.byId.get(node);
    const out: MenuEntry[] = [];
    const parent = self?.parent_id ? ix.byId.get(self.parent_id) : undefined;
    if (last && parent) out.push(entry("up", parent));
    for (const n of siblings(ix, node)) out.push(entry("near", n));
    if (last) for (const n of childrenOf(ix, node)) out.push(entry("inside", n));
    return out;
  }, [tree, node, last, locale]);

  return (
    <CrumbMenuView
      open={open}
      onOpenChange={setOpen}
      current={node}
      name={name}
      entries={all}
      loading={loading && !tree}
      error={!tree ? error : null}
      truncated={!!tree?.truncated}
      labels={labels}
      top={labels.top}
    />
  );
}

type ViewProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  current: string;
  name: string;
  entries: MenuEntry[];
  loading: boolean;
  error: unknown;
  truncated?: boolean;
  labels: Labels;
  top: string;
};

export function CrumbMenuView({ open, onOpenChange, current: node, name, entries: all, loading, error, truncated, labels, top }: ViewProps) {
  const { errors, common } = useUiText();
  const router = useRouter();
  const listId = useId();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const list = useRef<HTMLUListElement>(null);

  const filterable = all.filter((i) => i.group !== "up").length >= FILTER_FROM;
  const items = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return all.slice(0, SHOWN);
    return all.filter((i) => i.group !== "up" && (i.name.toLowerCase().includes(q) || (i.slug ?? "").toLowerCase().includes(q))).slice(0, SHOWN);
  }, [all, query]);

  const go = (item: MenuEntry | undefined) => {
    if (!item) return;
    onOpenChange(false);
    router.push(item.href);
  };

  const onKey = (e: KeyboardEvent<HTMLElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive((a) => (items.length ? (a + step + items.length) % items.length : 0));
    } else if (e.key === "Home") {
      e.preventDefault();
      setActive(0);
    } else if (e.key === "End") {
      e.preventDefault();
      setActive(Math.max(0, items.length - 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      go(items[active]);
    }
  };

  const heading = { up: labels.up, near: labels.near, inside: node === "root" ? top : labels.inside };
  const activeId = items[active] ? `${listId}-${active}` : undefined;

  return (
    <P.Root
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        setQuery("");
        setActive(0);
      }}
    >
      <P.Trigger
        aria-haspopup="listbox"
        aria-label={format(labels.open, { name })}
        className="motion-control grid size-6 shrink-0 place-items-center rounded-md text-muted hover:bg-surface-3 hover:text-ink data-[state=open]:bg-surface-3 data-[state=open]:text-ink"
      >
        <ChevronDown aria-hidden="true" className="motion-control size-3.5 in-data-[state=open]:rotate-180" />
      </P.Trigger>
      <P.Portal>
        <P.Content
          align="start"
          sideOffset={8}
          collisionPadding={8}
          onOpenAutoFocus={(e) => {
            e.preventDefault();
            (filterable ? (e.currentTarget as HTMLElement).querySelector("input") : list.current)?.focus();
          }}
          onKeyDown={onKey}
          className="motion-pop z-50 w-80 max-w-[calc(100vw-1rem)] overflow-hidden rounded-xl border border-line bg-surface/95 shadow-pop backdrop-blur-xl"
        >
          {filterable && (
            <div className="border-b border-line p-2">
              <input
                type="search"
                role="combobox"
                aria-expanded="true"
                aria-controls={listId}
                aria-activedescendant={activeId}
                aria-label={labels.filter}
                placeholder={labels.filter}
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setActive(0);
                }}
                className="h-8 w-full rounded-md border border-field bg-surface px-2.5 text-sm text-ink placeholder:text-muted focus-visible:border-ring"
              />
            </div>
          )}
          {loading ? (
            <p className="flex items-center gap-2 px-3 py-3 text-sm text-muted">
              <Spinner />
              {common.loading}
            </p>
          ) : error ? (
            <p className="px-3 py-3 text-sm text-danger">{errorText(errors, errorCode(error))}</p>
          ) : (
            <ul
              ref={list}
              id={listId}
              role="listbox"
              tabIndex={filterable ? -1 : 0}
              aria-label={format(labels.open, { name })}
              aria-activedescendant={filterable ? undefined : activeId}
              className="max-h-[60dvh] overflow-y-auto p-1 focus:outline-none"
            >
              {items.length === 0 ? (
                <li className="px-2.5 py-2 text-sm text-muted">{query ? labels.noMatch : common.nothingFound}</li>
              ) : (
                items.map((item, i) => {
                  const first = i === 0 || items[i - 1]?.group !== item.group;
                  const current = item.id === node;
                  return (
                    <li key={item.key} role="presentation">
                      {first && (
                        <p role="presentation" className="px-2.5 pt-2 pb-1 text-2xs font-semibold tracking-wide text-muted uppercase">
                          {heading[item.group]}
                        </p>
                      )}
                      <div
                        id={`${listId}-${i}`}
                        role="option"
                        aria-selected={current}
                        onMouseEnter={() => setActive(i)}
                        onClick={() => go(item)}
                        className={cn(
                          "motion-control flex cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-sm text-ink select-none",
                          i === active && "bg-surface-3",
                        )}
                      >
                        {item.group === "up" ? <CornerLeftUp aria-hidden="true" className="size-4 shrink-0 text-muted" /> : item.icon}
                        <span className="min-w-0 flex-1 truncate">
                          {item.group === "up" ? format(labels.upTo, { name: item.name }) : item.name}
                        </span>
                        {current && <Check aria-hidden="true" className="size-4 shrink-0 text-signal" />}
                      </div>
                    </li>
                  );
                })
              )}
            </ul>
          )}
          {truncated && <p className="border-t border-line px-3 py-2 text-xs text-muted">{labels.truncated}</p>}
        </P.Content>
      </P.Portal>
    </P.Root>
  );
}
