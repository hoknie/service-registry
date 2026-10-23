"use client";

import { X } from "lucide-react";
import { useId, useState } from "react";

import { format } from "@/i18n/format";
import { apiGet, type CatalogTable } from "@/lib/api";
import type { Suggestion } from "@/lib/labelSuggest";
import { nodeHits, nodeSearchQuery } from "@/lib/nodeSearch";
import { TOKEN_MAX_NODES, type PickedNode } from "@/lib/tokenNodes";

import { Input } from "../ui/Field";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";
import type { TokenLabels } from "./TokenTable";

export type NodeScopeLabels = Pick<
  TokenLabels["dialog"],
  "access" | "accessAll" | "accessNodes" | "nodesSearch" | "nodesHint" | "nodesPicked" | "removeNode"
>;

type Props = {
  value: PickedNode[];
  onChange: (value: PickedNode[]) => void;
  labels: NodeScopeLabels;
};

async function findNodes(draft: string, signal: AbortSignal): Promise<Suggestion[]> {
  const path = nodeSearchQuery(draft);
  if (!path) return [];
  const table = await apiGet<CatalogTable>(path as `/${string}`, signal);
  return nodeHits(table)
    .filter((h) => h.node.access === "read")
    .map((h) => ({ value: h.node.id, label: h.path ? `${h.path} / ${h.node.name}` : h.node.name, detail: h.node.kind }));
}

export function TokenNodes({ value, onChange, labels: t }: Props) {
  const id = useId();
  const [draft, setDraft] = useState("");
  const [focused, setFocused] = useState(false);
  const s = useSuggestions(draft, findNodes, focused);
  const full = value.length >= TOKEN_MAX_NODES;

  const choose = (picked: Suggestion) => {
    if (!value.some((n) => n.id === picked.value) && !full) onChange([...value, { id: picked.value, label: picked.label }]);
    setDraft("");
    s.close();
  };

  return (
    <div className="grid gap-2">
      {value.length > 0 && (
        <ul className="flex flex-wrap gap-1.5" aria-label={t.nodesPicked}>
          {value.map((n) => (
            <li key={n.id} className="inline-flex items-center gap-1 rounded-md border border-line bg-surface-2 py-0.5 pr-1 pl-2 text-sm text-ink">
              <span className="break-all">{n.label}</span>
              <button
                type="button"
                className="motion-control rounded p-0.5 text-muted hover:bg-surface-3 hover:text-ink focus-visible:outline-2 focus-visible:outline-signal"
                aria-label={format(t.removeNode, { name: n.label })}
                onClick={() => onChange(value.filter((v) => v.id !== n.id))}
              >
                <X aria-hidden="true" className="size-3.5" />
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="relative">
        <Input
          id={id}
          {...s.inputProps}
          aria-label={t.nodesSearch}
          placeholder={t.nodesSearch}
          disabled={full}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          onKeyDown={(e) => s.onKey(e, choose)}
        />
        {s.open && <SuggestionList id={s.listId} items={s.items} active={s.active} onActive={s.setActive} onChoose={choose} />}
      </div>
      <p className="text-xs text-muted">{format(t.nodesHint, { n: TOKEN_MAX_NODES })}</p>
    </div>
  );
}
