"use client";

import { User } from "lucide-react";
import { useCallback, useState } from "react";

import type { MemberCandidate } from "@/lib/api";
import type { Suggestion } from "@/lib/labelSuggest";
import { memberCandidates } from "@/lib/managedGroups";

import { Input } from "../ui/Field";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";
import { useUiText } from "../UiText";

type Props = {
  groupId: string;
  label: string;
  placeholder: string;
  onPick: (userId: string) => void;
  disabled?: boolean;
  candidates?: (q: string, signal: AbortSignal) => Promise<MemberCandidate[]>;
  minChars?: number;
};

export function MemberPicker({ groupId, label, placeholder, onPick, disabled, candidates, minChars = 0 }: Props) {
  const { common } = useUiText();
  const [draft, setDraft] = useState("");
  const [focused, setFocused] = useState(false);
  const suggest = useCallback(
    async (q: string, signal: AbortSignal): Promise<Suggestion[]> =>
      (await (candidates ?? ((query, s) => memberCandidates(groupId, query, s)))(q, signal)).map((c) => ({ value: c.id, label: c.display_name, detail: c.email })),
    [groupId, candidates],
  );
  const hints = useSuggestions(draft, suggest, focused);
  const choose = (s: Suggestion) => {
    onPick(s.value);
    setDraft("");
    hints.close();
  };
  return (
    <div className="relative">
      <Input
        {...hints.inputProps}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => hints.onKey(e, choose)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        placeholder={placeholder}
        aria-label={label}
        autoComplete="off"
        disabled={disabled}
        maxLength={100}
        className="w-full"
      />
      {hints.open && (
        <SuggestionList
          id={hints.listId}
          items={hints.items}
          active={hints.active}
          onActive={hints.setActive}
          onChoose={choose}
          icon={() => <User aria-hidden="true" className="size-4 shrink-0 text-muted" />}
        />
      )}
      {hints.empty && draft.trim().length >= minChars && (
        <p role="status" className="absolute top-full left-0 z-50 mt-1.5 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-muted shadow-pop">
          {common.nothingFound}
        </p>
      )}
    </div>
  );
}
