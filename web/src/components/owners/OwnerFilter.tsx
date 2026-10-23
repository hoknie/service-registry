"use client";

import { User, UsersRound, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { cn } from "@/lib/cn";
import type { Suggestion } from "@/lib/labelSuggest";
import { ownerFilter } from "@/lib/owners";
import { suggestOwners } from "@/lib/ownersApi";

import { Input } from "../ui/Field";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";

type Labels = { owner: string; anyOwner: string; mine: string };
type Props = { value: string; onChange: (v: string) => void; labels: Labels; className?: string };

export function OwnerFilter({ value, onChange, labels, className }: Props) {
  const [text, setText] = useState(value === "me" ? labels.mine : "");
  const [focused, setFocused] = useState(false);

  useEffect(() => {
    if (value === "" || value === "me") {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setText(value === "me" ? labels.mine : "");
      return;
    }
    const abort = new AbortController();
    suggestOwners("", abort.signal)
      .then((items) => {
        const found = items.find((o) => ownerFilter(o.subject_kind, o.subject_id) === value);
        if (found) setText(found.name);
      })
      .catch(() => undefined);
    return () => abort.abort();
  }, [value, labels.mine]);

  const suggest = useCallback(
    async (q: string, signal: AbortSignal): Promise<Suggestion[]> => {
      const draft = q === labels.mine ? "" : q;
      const found = await suggestOwners(draft, signal);
      return [
        { value: "me", label: labels.mine },
        ...found.map((o) => ({ value: ownerFilter(o.subject_kind, o.subject_id), label: o.name, detail: String(o.count) })),
      ];
    },
    [labels.mine],
  );
  const hints = useSuggestions(text, suggest, focused);
  const choose = (s: Suggestion) => {
    setText(s.label);
    onChange(s.value);
    hints.close();
  };

  return (
    <div className={cn("relative", className)}>
      <Input
        {...hints.inputProps}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => hints.onKey(e, choose)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        placeholder={labels.anyOwner}
        aria-label={labels.owner}
        autoComplete="off"
        maxLength={100}
        className={cn("w-full", value && "pr-8")}
      />
      {value && (
        <button
          type="button"
          onClick={() => {
            setText("");
            onChange("");
          }}
          aria-label={`${labels.owner}: ${labels.anyOwner}`}
          className="motion-control absolute top-1/2 right-1.5 grid size-6 -translate-y-1/2 place-items-center rounded text-muted hover:bg-surface-3 hover:text-ink"
        >
          <X aria-hidden="true" className="size-4" />
        </button>
      )}
      {hints.open && (
        <SuggestionList
          id={hints.listId}
          items={hints.items}
          active={hints.active}
          onActive={hints.setActive}
          onChoose={choose}
          icon={(s) =>
            s.value.startsWith("user:") || s.value === "me" ? (
              <User aria-hidden="true" className="size-4 shrink-0 text-muted" />
            ) : (
              <UsersRound aria-hidden="true" className="size-4 shrink-0 text-muted" />
            )
          }
        />
      )}
    </div>
  );
}
