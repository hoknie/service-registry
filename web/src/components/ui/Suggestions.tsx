"use client";

import { useEffect, useId, useState, type KeyboardEvent } from "react";

import { cn } from "@/lib/cn";
import type { Suggestion } from "@/lib/labelSuggest";

import { useUiText } from "../UiText";

export type Suggest = (draft: string, signal: AbortSignal) => Promise<Suggestion[]>;

export function useSuggestions(draft: string, suggest: Suggest | undefined, focused: boolean) {
  const listId = useId();
  const [items, setItems] = useState<Suggestion[]>([]);
  const [active, setActive] = useState(0);
  const [closed, setClosed] = useState(false);

  useEffect(() => {
    if (!suggest || !focused) return;
    const abort = new AbortController();
    const timer = setTimeout(() => {
      suggest(draft, abort.signal)
        .then((found) => {
          if (abort.signal.aborted) return;
          setItems(found);
          setActive(0);
          setClosed(false);
        })
        .catch(() => !abort.signal.aborted && setItems([]));
    }, 200);
    return () => {
      abort.abort();
      clearTimeout(timer);
    };
  }, [draft, suggest, focused]);

  const open = !!suggest && focused && !closed && items.length > 0;

  const onKey = (e: KeyboardEvent<HTMLInputElement>, choose: (s: Suggestion) => void): boolean => {
    if (!open) return false;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive((a) => (a + step + items.length) % items.length);
      return true;
    }
    if (e.key === "Enter") {
      const s = items[active];
      if (!s) return false;
      e.preventDefault();
      choose(s);
      return true;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      setClosed(true);
      return true;
    }
    return false;
  };

  const inputProps = suggest
    ? {
        role: "combobox" as const,
        "aria-expanded": open,
        "aria-controls": listId,
        "aria-autocomplete": "list" as const,
        "aria-activedescendant": open ? `${listId}-${active}` : undefined,
      }
    : {};

  return { listId, items, active, setActive, open, onKey, inputProps, close: () => setClosed(true) };
}

type ListProps = {
  id: string;
  items: Suggestion[];
  active: number;
  onActive: (i: number) => void;
  onChoose: (s: Suggestion) => void;
};

export function SuggestionList({ id, items, active, onActive, onChoose }: ListProps) {
  const { common } = useUiText();
  return (
    <ul
      id={id}
      role="listbox"
      aria-label={common.suggestions}
      className="motion-pop absolute top-full left-0 z-50 mt-1.5 max-h-64 w-full min-w-56 overflow-y-auto rounded-lg border border-line bg-surface p-1 shadow-pop"
    >
      {items.map((s, i) => (
        <li
          key={`${s.value}-${i}`}
          id={`${id}-${i}`}
          role="option"
          aria-selected={i === active}
          onMouseDown={(e) => e.preventDefault()}
          onMouseEnter={() => onActive(i)}
          onClick={() => onChoose(s)}
          className={cn(
            "motion-control flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 font-mono text-sm text-ink select-none",
            i === active && "bg-surface-3",
          )}
        >
          <span className="min-w-0 flex-1 truncate">{s.label}</span>
          {s.detail && <span className="shrink-0 text-xs text-muted">{s.detail}</span>}
        </li>
      ))}
    </ul>
  );
}
