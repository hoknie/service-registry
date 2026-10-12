"use client";

import { ChevronDown, Search } from "lucide-react";
import { Popover as P } from "radix-ui";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react";

import { cn } from "@/lib/cn";
import { iconNames } from "@/lib/iconSet";
import { searchIcons } from "@/lib/linkKindForm";

import { Input } from "./Field";
import { SetIcon } from "./SetIcon";

const COLUMNS = 8;

type Labels = { search: string; empty: string; choose: string };

type Props = {
  value: string;
  onChange: (name: string) => void;
  labels: Labels;
  id?: string;
  className?: string;
};

export function IconPicker({ value, onChange, labels, id, className }: Props) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [names, setNames] = useState<string[] | null>(null);
  const grid = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open || names) return;
    let live = true;
    void iconNames().then((all) => live && setNames(all));
    return () => {
      live = false;
    };
  }, [open, names]);

  const found = useMemo(() => (names ? searchIcons(names, query) : []), [names, query]);

  const focusAt = (i: number) => {
    const buttons = grid.current?.querySelectorAll<HTMLButtonElement>("button");
    if (!buttons || buttons.length === 0) return;
    buttons[Math.max(0, Math.min(i, buttons.length - 1))]?.focus();
  };

  const onGridKey = (e: KeyboardEvent<HTMLDivElement>) => {
    const buttons = [...(grid.current?.querySelectorAll<HTMLButtonElement>("button") ?? [])];
    const i = buttons.indexOf(document.activeElement as HTMLButtonElement);
    if (i < 0) return;
    const step = { ArrowRight: 1, ArrowLeft: -1, ArrowDown: COLUMNS, ArrowUp: -COLUMNS }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    focusAt(i + step);
  };

  const choose = (name: string) => {
    onChange(name);
    setOpen(false);
    setQuery("");
  };

  return (
    <P.Root open={open} onOpenChange={setOpen}>
      <P.Trigger asChild>
        <button
          id={id}
          type="button"
          aria-label={`${labels.choose}: ${value}`}
          className={cn(
            "motion-control flex h-9 w-full items-center gap-2 rounded-md border border-field bg-surface px-2.5 text-sm text-ink hover:border-ink-2 focus-visible:border-ring",
            className,
          )}
        >
          <SetIcon name={value} className="text-ink-2" />
          <span className="min-w-0 flex-1 truncate text-left font-mono">{value}</span>
          <ChevronDown aria-hidden="true" className="size-4 text-muted" />
        </button>
      </P.Trigger>
      <P.Portal>
        <P.Content
          align="start"
          sideOffset={4}
          collisionPadding={8}
          className="motion-pop z-50 w-[min(22rem,calc(100vw-2rem))] rounded-lg border border-line bg-surface p-2 shadow-pop"
          onOpenAutoFocus={(e) => {
            e.preventDefault();
            (e.currentTarget as HTMLElement).querySelector("input")?.focus();
          }}
        >
          <div className="relative mb-2">
            <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted" />
            <Input
              aria-label={labels.search}
              placeholder={labels.search}
              spellCheck={false}
              className="pl-8 font-mono"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "ArrowDown") {
                  e.preventDefault();
                  focusAt(0);
                }
                if (e.key === "Enter" && found[0]) {
                  e.preventDefault();
                  choose(found[0]);
                }
              }}
            />
          </div>
          {names && found.length === 0 && <p className="px-1 py-3 text-center text-sm text-muted">{labels.empty}</p>}
          <div ref={grid} role="group" aria-label={labels.choose} onKeyDown={onGridKey} className="grid grid-cols-8 gap-1">
            {found.map((name) => (
              <button
                key={name}
                type="button"
                title={name}
                aria-label={name}
                aria-pressed={name === value}
                onClick={() => choose(name)}
                className={cn(
                  "motion-control grid size-9 place-items-center rounded-md text-ink-2 hover:bg-surface-2",
                  name === value && "bg-signal-soft text-ink ring-1 ring-signal ring-inset",
                )}
              >
                <SetIcon name={name} />
              </button>
            ))}
          </div>
        </P.Content>
      </P.Portal>
    </P.Root>
  );
}
