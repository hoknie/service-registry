"use client";

import { Search } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import type { Suggestion } from "@/lib/labelSuggest";

import { Button } from "../ui/Button";
import { Input } from "../ui/Field";
import { TagInput } from "../ui/TagInput";

type Props = {
  q: string;
  tags: string[];
  onChange: (next: { q?: string; tags?: string[] }) => void;
  placeholder: string;
  tagsLabel: string;
  suggest?: (draft: string, signal: AbortSignal) => Promise<Suggestion[]>;
  extra?: ReactNode;
  reset: { shown: boolean; label: string; onReset: () => void };
};

export function PlaceSearch({ q, tags, onChange, placeholder, tagsLabel, suggest, extra, reset }: Props) {
  const [query, setQuery] = useState(q);
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setQuery(q);
  }, [q]);
  return (
    <form
      role="search"
      className="flex flex-wrap items-center gap-2"
      onSubmit={(e) => {
        e.preventDefault();
        onChange({ q: query.trim() });
      }}
    >
      <label className="relative w-72 max-w-full">
        <span className="sr-only">{placeholder}</span>
        <Search aria-hidden="true" className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted" />
        <Input className="w-full pl-9" type="search" placeholder={placeholder} value={query} onChange={(e) => setQuery(e.target.value)} />
      </label>
      <TagInput
        className="w-72 max-w-full py-1"
        aria-label={tagsLabel}
        placeholder={tagsLabel}
        value={tags.join("\n")}
        onChange={(v) => onChange({ tags: v.split("\n").filter(Boolean) })}
        suggest={suggest}
      />
      {extra}
      {reset.shown && (
        <Button
          variant="ghost"
          onClick={() => {
            setQuery("");
            reset.onReset();
          }}
        >
          {reset.label}
        </Button>
      )}
    </form>
  );
}
