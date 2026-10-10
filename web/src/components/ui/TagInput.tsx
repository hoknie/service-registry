"use client";

import { X } from "lucide-react";
import { useState, type ClipboardEvent, type KeyboardEvent, type ReactNode } from "react";

import { format } from "@/i18n/format";
import { cn } from "@/lib/cn";
import { suggestLabels } from "@/lib/labelsApi";
import { pickLabel, type Suggestion } from "@/lib/labelSuggest";
import { addTags, fromTags, parseKeyValue, parseLabel, splitTags, toTags } from "@/lib/tags";

import { useUiText } from "../UiText";
import { SuggestionList, useSuggestions, type Suggest } from "./Suggestions";
import { Tooltip } from "./Tooltip";

type Props = {
  value: string;
  onChange: (value: string) => void;
  onDraft?: (draft: string) => void;
  validate?: (tag: string) => string | null;
  render?: (tag: string) => ReactNode;
  suggest?: Suggest;
  pick?: (draft: string, s: Suggestion) => { draft: string; commit: boolean };
  id?: string;
  placeholder?: string;
  disabled?: boolean;
  mono?: boolean;
  className?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: boolean | "true" | "false";
};

export function hasInvalidTags(value: string, validate: (tag: string) => string | null): boolean {
  return toTags(value).some((t) => validate(t) !== null);
}

export function TagInput({ value, onChange, onDraft, validate, render, suggest, pick, id, placeholder, disabled, mono = true, className, ...aria }: Props) {
  const { common } = useUiText();
  const [draft, setDraftState] = useState("");
  const [focused, setFocused] = useState(false);
  const tags = toTags(value);
  const hints = useSuggestions(draft, suggest, focused);
  const setDraft = (next: string) => {
    setDraftState(next);
    onDraft?.(next);
  };

  const commit = (text: string) => {
    const incoming = splitTags(text);
    if (incoming.length) onChange(fromTags(addTags(tags, incoming)));
    setDraft("");
  };
  const remove = (tag: string) => onChange(fromTags(tags.filter((t) => t !== tag)));
  const choose = (s: Suggestion) => {
    const next = pick ? pick(draft, s) : { draft: s.value, commit: true };
    if (next.commit) commit(next.draft);
    else setDraft(next.draft);
  };

  const onKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (hints.onKey(e, choose)) return;
    if (e.key === "Enter" || (e.key === "," && !/\{[^}]*$/.test(draft))) {
      e.preventDefault();
      commit(draft);
    } else if (e.key === "Backspace" && draft === "" && tags.length) {
      e.preventDefault();
      remove(tags[tags.length - 1]!);
    }
  };

  const onPaste = (e: ClipboardEvent<HTMLInputElement>) => {
    const text = e.clipboardData.getData("text");
    if (/[\n,]/.test(text)) {
      e.preventDefault();
      commit(draft + text);
    }
  };

  return (
    <div
      className={cn(
        "motion-control relative flex min-h-9 w-full min-w-0 flex-wrap items-center gap-1.5 rounded-md border border-field bg-surface px-2 py-1.5",
        "focus-within:border-ring hover:border-ink-2",
        disabled && "opacity-60",
        className,
      )}
    >
      {tags.map((tag) => {
        const error = validate?.(tag) ?? null;
        return (
          <Tooltip key={tag} content={error}>
            <span
              aria-invalid={error ? true : undefined}
              className={cn(
                "inline-flex max-w-full animate-pop-in items-center gap-1 rounded-md border px-2 py-0.5 text-sm",
                mono && "font-mono",
                error ? "border-danger bg-danger-soft text-danger" : "border-line bg-surface-2 text-ink",
              )}
            >
              <span className="min-w-0 truncate">{render ? render(tag) : tag}</span>
              {!disabled && (
                <button
                  type="button"
                  onClick={() => remove(tag)}
                  aria-label={format(common.removeTag, { value: tag })}
                  className="motion-control -mr-1 grid size-5 shrink-0 place-items-center rounded text-muted hover:bg-surface-3 hover:text-ink"
                >
                  <X aria-hidden="true" className="size-3.5" />
                </button>
              )}
            </span>
          </Tooltip>
        );
      })}
      <input
        id={id}
        {...aria}
        {...hints.inputProps}
        disabled={disabled}
        value={draft}
        placeholder={tags.length ? undefined : placeholder}
        spellCheck={false}
        autoComplete="off"
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKey}
        onPaste={onPaste}
        onFocus={() => setFocused(true)}
        onBlur={() => {
          setFocused(false);
          if (draft.trim()) commit(draft);
        }}
        className={cn("h-7 min-w-24 flex-1 bg-transparent px-1 text-base text-ink outline-none placeholder:text-muted", mono && "font-mono text-sm")}
      />
      {hints.open && <SuggestionList id={hints.listId} items={hints.items} active={hints.active} onActive={hints.setActive} onChoose={choose} />}
    </div>
  );
}

export function keyValueError(tag: string, message: string): string | null {
  return parseKeyValue(tag) ? null : message;
}

export function KeyValueInput(props: Omit<Props, "validate" | "render">) {
  const { common } = useUiText();
  return (
    <TagInput
      {...props}
      validate={(t) => keyValueError(t, common.keyValueFormat)}
      render={(t) => {
        const kv = parseKeyValue(t);
        if (!kv) return t;
        return (
          <>
            <span className="text-muted">{kv.key}:</span> {kv.value}
          </>
        );
      }}
    />
  );
}

export function labelError(tag: string, message: string): string | null {
  return parseLabel(tag) ? null : message;
}

export function LabelsInput(props: Omit<Props, "validate" | "render" | "suggest" | "pick">) {
  const { common } = useUiText();
  return (
    <TagInput
      {...props}
      suggest={suggestLabels}
      pick={pickLabel}
      validate={(t) => labelError(t, common.labelFormat)}
      render={(t) => {
        const l = parseLabel(t);
        if (!l || !l.value) return t;
        return (
          <>
            <span className="text-muted">{l.key}:</span> {l.value}
          </>
        );
      }}
    />
  );
}
