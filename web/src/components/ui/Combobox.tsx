"use client";

import { Check, ChevronDown } from "lucide-react";
import { Popover as P } from "radix-ui";
import { useId, useMemo, useRef, useState, type KeyboardEvent } from "react";

import { filterOptions, splitHighlight, type ComboOption } from "@/lib/combobox";
import { cn } from "@/lib/cn";

import { useUiText } from "../UiText";

type Props = {
  value: string;
  onChange: (value: string) => void;
  options: ComboOption[];
  id?: string;
  placeholder?: string;
  allowCustom?: boolean;
  disabled?: boolean;
  required?: boolean;
  inputType?: "text" | "email";
  className?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean | "true" | "false";
};

export function Combobox({ value, onChange, options, id, placeholder, allowCustom, disabled, required, inputType = "text", className, ...aria }: Props) {
  const { common } = useUiText();
  const listId = useId();
  const selected = options.find((o) => o.value === value);
  const [query, setQuery] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const text = query ?? (selected ? selected.label : allowCustom ? value : "");
  const matches = useMemo(() => filterOptions(options, query ?? ""), [options, query]);

  const choose = (v: string) => {
    onChange(v);
    setQuery(null);
    setOpen(false);
  };

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) setOpen(true);
      const step = e.key === "ArrowDown" ? 1 : -1;
      setActive((a) => (matches.length ? (a + step + matches.length) % matches.length : 0));
    } else if (e.key === "Home" && open) {
      e.preventDefault();
      setActive(0);
    } else if (e.key === "End" && open) {
      e.preventDefault();
      setActive(Math.max(0, matches.length - 1));
    } else if (e.key === "Enter" && open) {
      const m = matches[active];
      if (m) {
        e.preventDefault();
        choose(m.option.value);
      } else if (!allowCustom) e.preventDefault();
    } else if (e.key === "Escape" && open) {
      e.preventDefault();
      setOpen(false);
      setQuery(null);
    }
  };

  const activeId = open && matches[active] ? `${listId}-${active}` : undefined;

  return (
    <P.Root open={open && !disabled} onOpenChange={setOpen}>
      <P.Anchor asChild>
        <div className="relative min-w-0">
          <input
            ref={input}
            id={id}
            {...aria}
            type={inputType}
            role="combobox"
            aria-expanded={open}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={activeId}
            autoComplete="off"
            spellCheck={false}
            disabled={disabled}
            required={required}
            placeholder={placeholder}
            value={text}
            onFocus={() => setOpen(true)}
            onClick={() => setOpen(true)}
            onBlur={() => {
              if (allowCustom && query !== null) onChange(query.trim());
              setQuery(null);
            }}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
              setOpen(true);
              if (allowCustom) onChange(e.target.value);
              else if (selected) onChange("");
            }}
            onKeyDown={onKey}
            className={cn(
              "motion-control h-9 w-full rounded-md border border-field bg-surface pr-9 pl-3 text-base text-ink placeholder:text-muted",
              "hover:border-ink-2 focus-visible:border-ring disabled:opacity-60 aria-invalid:border-danger",
              className,
            )}
          />
          <ChevronDown aria-hidden="true" className={cn("pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-muted motion-control", open && "rotate-180")} />
        </div>
      </P.Anchor>
      <P.Portal>
        <P.Content
          align="start"
          sideOffset={6}
          collisionPadding={8}
          onOpenAutoFocus={(e) => e.preventDefault()}
          onInteractOutside={(e) => {
            if (e.target instanceof Node && input.current?.contains(e.target)) e.preventDefault();
          }}
          className="motion-pop z-50 w-[var(--radix-popover-trigger-width)] min-w-56 overflow-hidden rounded-lg border border-line bg-surface shadow-pop"
        >
          <ul id={listId} role="listbox" className="max-h-72 overflow-y-auto p-1">
            {matches.length === 0 ? (
              <li className="px-2.5 py-2 text-sm text-muted">{common.nothingFound}</li>
            ) : (
              matches.map((m, i) => {
                const [before, hit, after] = splitHighlight(m.option.label, m.start, m.end);
                const isSelected = m.option.value === value;
                return (
                  <li
                    key={m.option.value}
                    id={`${listId}-${i}`}
                    role="option"
                    aria-selected={isSelected}
                    onMouseDown={(e) => e.preventDefault()}
                    onMouseEnter={() => setActive(i)}
                    onClick={() => choose(m.option.value)}
                    className={cn(
                      "motion-control flex cursor-pointer items-center gap-2 rounded-md px-2.5 py-1.5 text-sm text-ink select-none",
                      i === active && "bg-surface-3",
                    )}
                  >
                    <span className="min-w-0 flex-1">
                      <span className="block truncate">
                        {before}
                        {hit && <mark className="rounded-sm bg-signal-soft px-0.5 text-signal">{hit}</mark>}
                        {after}
                      </span>
                      {m.option.detail && <span className="block truncate text-xs text-muted">{m.option.detail}</span>}
                    </span>
                    {isSelected && <Check aria-hidden="true" className="size-4 shrink-0 text-signal" />}
                  </li>
                );
              })
            )}
          </ul>
        </P.Content>
      </P.Portal>
    </P.Root>
  );
}
