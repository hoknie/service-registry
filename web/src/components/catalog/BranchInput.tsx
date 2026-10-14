"use client";

import { useCallback, useRef, useState, type InputHTMLAttributes } from "react";

import { apiGet, type Branch, type Page } from "@/lib/api";
import { branchSuggestions } from "@/lib/branchSuggest";
import type { Suggestion } from "@/lib/labelSuggest";
import { cn } from "@/lib/cn";

import { Input } from "../ui/Field";
import { SuggestionList, useSuggestions, type Suggest } from "../ui/Suggestions";

type Props = Omit<InputHTMLAttributes<HTMLInputElement>, "value" | "onChange"> & {
  value: string;
  onChange: (v: string) => void;
  suggest?: Suggest;
};

export function BranchInput({ value, onChange, suggest, readOnly, className, ...rest }: Props) {
  const [focused, setFocused] = useState(false);
  const picked = useRef<string | null>(null);
  const guarded = useCallback<Suggest>(
    (draft, signal) => (suggest && draft !== picked.current ? suggest(draft, signal) : Promise.resolve([])),
    [suggest],
  );
  const hints = useSuggestions(value, readOnly || !suggest ? undefined : guarded, focused);
  const choose = (s: Suggestion) => {
    picked.current = s.value;
    onChange(s.value);
    hints.close();
  };
  return (
    <div className="relative min-w-0">
      <Input
        {...rest}
        {...hints.inputProps}
        readOnly={readOnly}
        autoComplete="off"
        spellCheck={false}
        className={cn("font-mono", className)}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onKeyDown={(e) => hints.onKey(e, choose)}
      />
      {hints.open && <SuggestionList id={hints.listId} items={hints.items} active={hints.active} onActive={hints.setActive} onChoose={choose} />}
    </div>
  );
}

export function projectBranches(projectId: string, def: string | null, defaultLabel: string): Suggest {
  return async (draft, signal) => {
    const query = new URLSearchParams({ state: "all", limit: "20" });
    if (draft.trim()) query.set("q", draft.trim());
    const page = await apiGet<Page<Branch>>(`/v1/catalog/nodes/${projectId}/branches?${query}` as `/${string}`, signal);
    return branchSuggestions(page.items.map((b) => b.name), draft, page.items.find((b) => b.is_default)?.name ?? def, defaultLabel);
  };
}

export function knownBranches(names: string[], def: string | null, defaultLabel: string): Suggest {
  return async (draft) => branchSuggestions(names, draft, def, defaultLabel);
}
