"use client";

import { User, UsersRound } from "lucide-react";
import { useCallback, useState } from "react";

import type { Suggestion } from "@/lib/labelSuggest";
import type { DraftOwner } from "@/lib/owners";
import { ownerCandidates } from "@/lib/ownersApi";

import { Input } from "../ui/Field";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";

type Props = { nodeId: string; label: string; placeholder: string; onPick: (o: DraftOwner) => void; disabled?: boolean };

export function OwnerPicker({ nodeId, label, placeholder, onPick, disabled }: Props) {
  const [draft, setDraft] = useState("");
  const [focused, setFocused] = useState(false);
  const suggest = useCallback(
    async (q: string, signal: AbortSignal): Promise<Suggestion[]> =>
      (await ownerCandidates(nodeId, q, signal)).map((c) => ({
        value: `${c.subject_kind}:${c.subject_kind === "group" ? c.name : (c.email ?? c.name)}`,
        label: c.name,
        detail: c.email ?? undefined,
      })),
    [nodeId],
  );
  const hints = useSuggestions(draft, suggest, focused);
  const choose = (s: Suggestion) => {
    const at = s.value.indexOf(":");
    const kind = s.value.slice(0, at) === "group" ? "group" : "user";
    onPick({ subject_kind: kind, subject: s.value.slice(at + 1), name: s.label, detail: s.detail ?? null });
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
          icon={(s) =>
            s.value.startsWith("group:") ? (
              <UsersRound aria-hidden="true" className="size-4 shrink-0 text-muted" />
            ) : (
              <User aria-hidden="true" className="size-4 shrink-0 text-muted" />
            )
          }
        />
      )}
    </div>
  );
}
