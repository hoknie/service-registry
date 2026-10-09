"use client";

import { ChevronRight } from "lucide-react";
import { useEffect, useState } from "react";

import { format } from "@/i18n/format";
import { cn } from "@/lib/cn";
import { entries, leafText, parseJson, valueType, type StructuredKind, type ValueType } from "@/lib/structured";

import { Button } from "../ui/Button";
import type { CatalogLabels } from "../catalog/shared";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../ui/Collapsible";

type Labels = CatalogLabels["docs"]["viewer"];

type Expansion = { mode: "default" | "all" | "none"; n: number };

const DEFAULT_DEPTH = 2;

const leafTone: Record<ValueType, string> = {
  string: "text-signal",
  number: "text-cobalt",
  boolean: "text-amber",
  null: "text-muted italic",
  object: "",
  array: "",
};

function ValueNode({ name, value, depth, expansion, labels: t }: { name: string | null; value: unknown; depth: number; expansion: Expansion; labels: Labels }) {
  const type = valueType(value);
  const container = type === "object" || type === "array";
  const [open, setOpen] = useState(expansion.mode === "all" || (expansion.mode === "default" && depth < DEFAULT_DEPTH));
  const key = name !== null && <span className="text-ink">{name}</span>;
  if (!container) {
    return (
      <li className="min-w-0 py-0.5 pl-5 break-all">
        {key}
        {key && <span className="text-muted">: </span>}
        <span className={leafTone[type]}>{leafText(value)}</span>
      </li>
    );
  }
  const items = entries(value);
  const [l, r] = type === "array" ? ["[", "]"] : ["{", "}"];
  return (
    <Collapsible asChild open={open} onOpenChange={setOpen}>
      <li className="min-w-0">
        <CollapsibleTrigger asChild>
          <button type="button" className="motion-control flex min-w-0 items-center gap-1 rounded py-0.5 text-left hover:bg-surface-2">
            <ChevronRight aria-hidden="true" className={cn("motion-control size-3.5 shrink-0 text-muted", open && "rotate-90")} />
            {key}
            {key && <span className="text-muted">:</span>}
            <span className="text-muted">
              {open ? l : `${l}…${r}`}
              {!open && <span className="ml-2 text-xs">{format(t.items, { n: items.length })}</span>}
            </span>
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <ul className="ml-[0.4rem] border-l border-line pl-2">
            {items.map(([k, v]) => (
              <ValueNode key={k} name={k} value={v} depth={depth + 1} expansion={expansion} labels={t} />
            ))}
          </ul>
          <span className="pl-5 text-muted">{r}</span>
        </CollapsibleContent>
      </li>
    </Collapsible>
  );
}

export function StructuredView({ kind, content, labels: t }: { kind: StructuredKind; content: string; labels: Labels }) {
  const [docs, setDocs] = useState<unknown[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [source, setSource] = useState(false);
  const [expansion, setExpansion] = useState<Expansion>({ mode: "default", n: 0 });

  useEffect(() => {
    let live = true;
    (async () => {
      try {
        if (kind === "json") return parseJson(content);
        const { parseAllDocuments } = await import("yaml");
        return parseAllDocuments(content).map((d) => {
          if (d.errors.length) throw d.errors[0];
          return d.toJS({ maxAliasCount: 1000 });
        });
      } catch {
        return null;
      }
    })().then((parsed) => {
      if (!live) return;
      setDocs(parsed);
      setFailed(parsed === null);
    });
    return () => {
      live = false;
    };
  }, [kind, content]);

  const raw = <pre className="overflow-x-auto rounded-md bg-surface-2 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink-2">{content}</pre>;
  if (failed) {
    return (
      <div className="grid gap-3">
        <p className="text-sm text-muted">{t.parseFailed}</p>
        {raw}
      </div>
    );
  }
  if (!docs) return raw;
  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap gap-2">
        <Button size="sm" aria-pressed={!source} variant={source ? "ghost" : "secondary"} onClick={() => setSource(false)}>
          {t.tree}
        </Button>
        <Button size="sm" aria-pressed={source} variant={source ? "secondary" : "ghost"} onClick={() => setSource(true)}>
          {t.source}
        </Button>
        {!source && (
          <>
            <Button size="sm" variant="ghost" onClick={() => setExpansion({ mode: "all", n: expansion.n + 1 })}>
              {t.expandAll}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setExpansion({ mode: "none", n: expansion.n + 1 })}>
              {t.collapseAll}
            </Button>
          </>
        )}
      </div>
      {source
        ? raw
        : docs.map((doc, i) => (
            <div key={`${i}:${expansion.n}`} className="min-w-0 overflow-x-auto rounded-md bg-surface-2 p-3 font-mono text-xs leading-relaxed">
              {docs.length > 1 && <p className="mb-1 font-sans text-xs font-medium text-muted">{format(t.document, { n: i + 1 })}</p>}
              <ul>
                <ValueNode name={null} value={doc} depth={0} expansion={expansion} labels={t} />
              </ul>
            </div>
          ))}
    </div>
  );
}
