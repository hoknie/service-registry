"use client";

import { User, UsersRound } from "lucide-react";
import { useCallback, useState } from "react";

import { format } from "@/i18n/format";
import type { Suggestion } from "@/lib/labelSuggest";
import { sameSubject, type DraftOwner } from "@/lib/owners";
import { ownerCandidates } from "@/lib/ownersApi";

import { Input } from "../ui/Field";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";
import { useUiText } from "../UiText";

type Props = {
  nodeId: string;
  label: string;
  placeholder: string;
  membersLabel: string;
  orgLabel?: string;
  exclude: DraftOwner[];
  onPick: (o: DraftOwner) => void;
  disabled?: boolean;
};

function draftOfValue(s: Suggestion): DraftOwner {
  const at = s.value.indexOf(":");
  const kind = s.value.slice(0, at) === "group" ? "group" : "user";
  return { subject_kind: kind, subject: s.value.slice(at + 1), name: s.label, detail: kind === "user" ? (s.detail ?? null) : null };
}

export function OwnerPicker({ nodeId, label, placeholder, membersLabel, orgLabel, exclude, onPick, disabled }: Props) {
  const { common } = useUiText();
  const [draft, setDraft] = useState("");
  const [focused, setFocused] = useState(false);
  const suggest = useCallback(
    async (q: string, signal: AbortSignal): Promise<Suggestion[]> =>
      (await ownerCandidates(nodeId, q, signal))
        .map((c) => ({
          value: `${c.subject_kind}:${c.subject_kind === "group" ? c.name : (c.email ?? c.name)}`,
          label: c.name,
          detail:
            c.subject_kind === "group"
              ? [format(membersLabel, { n: String(c.members ?? 0) }), c.organization && orgLabel].filter(Boolean).join(" · ")
              : (c.email ?? undefined),
        }))
        .filter((s) => !exclude.some((o) => sameSubject(o, draftOfValue(s)))),
    [nodeId, membersLabel, orgLabel, exclude],
  );
  const hints = useSuggestions(draft, suggest, focused);
  const choose = (s: Suggestion) => {
    onPick(draftOfValue(s));
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
      {hints.empty && (
        <p role="status" className="absolute top-full left-0 z-50 mt-1.5 w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-muted shadow-pop">
          {common.nothingFound}
        </p>
      )}
    </div>
  );
}
