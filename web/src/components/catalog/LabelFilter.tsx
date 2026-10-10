"use client";

import { useState } from "react";

import { cn } from "@/lib/cn";
import { suggestLabels } from "@/lib/labelsApi";
import { lastSegment, pickFilter, type Suggestion } from "@/lib/labelSuggest";

import { Input } from "../ui/Field";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";

type Props = { value: string; onChange: (v: string) => void; placeholder: string; label: string; hint: string; className?: string };

const suggestTail = (text: string, signal: AbortSignal) => suggestLabels(lastSegment(text).tail, signal);

export function LabelFilter({ value, onChange, placeholder, label, hint, className }: Props) {
  const [focused, setFocused] = useState(false);
  const hints = useSuggestions(value, suggestTail, focused);
  const choose = (s: Suggestion) => onChange(pickFilter(value, s));
  return (
    <div className={cn("relative", className)}>
      <Input
        {...hints.inputProps}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => hints.onKey(e, choose)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        placeholder={placeholder}
        aria-label={`${label}: ${hint}`}
        title={hint}
        autoComplete="off"
        spellCheck={false}
        className="w-full font-mono"
      />
      {hints.open && <SuggestionList id={hints.listId} items={hints.items} active={hints.active} onActive={hints.setActive} onChoose={choose} />}
    </div>
  );
}
