"use client";

import { ChevronRight } from "lucide-react";
import { useState } from "react";

import { schemaAnchor, type ApiSchema } from "@/lib/apis";

import type { ApiLabels } from "./ApiView";

type Props = { schema: ApiSchema | null; labels: ApiLabels; onRef: (name: string) => void; depth?: number };

function typeOf(s: ApiSchema): string {
  const t = Array.isArray(s.type) ? s.type.join(" | ") : typeof s.type === "string" ? s.type : "";
  return [t, typeof s.format === "string" ? `(${s.format})` : ""].filter(Boolean).join(" ");
}

export function SchemaRef({ schema, labels: t, onRef }: { schema: ApiSchema; labels: ApiLabels; onRef: (name: string) => void }) {
  const ref = schema.ref as string;
  if (schema.unresolved) {
    return (
      <span className="font-mono text-xs text-danger" title={t.refMissing}>
        {ref} · {t.refMissing}
      </span>
    );
  }
  return (
    <a
      href={`#${schemaAnchor(ref)}`}
      className="font-mono text-xs text-signal hover:underline"
      onClick={(e) => {
        e.preventDefault();
        onRef(ref);
      }}
    >
      {ref}
    </a>
  );
}

export function SchemaTree({ schema, labels: t, onRef, depth = 0 }: Props) {
  const [open, setOpen] = useState(depth < 2);
  if (!schema) return <span className="text-xs text-muted">—</span>;
  if (typeof schema.ref === "string") return <SchemaRef schema={schema} labels={t} onRef={onRef} />;
  const props = (schema.properties ?? {}) as Record<string, ApiSchema>;
  const required = new Set(Array.isArray(schema.required) ? (schema.required as string[]) : []);
  const items = schema.items && typeof schema.items === "object" ? (schema.items as ApiSchema) : null;
  const variants = (["one_of", "any_of", "all_of"] as const).filter((k) => Array.isArray(schema[k]));
  const names = Object.keys(props);
  const nested = names.length > 0 || items !== null || variants.length > 0;
  return (
    <div className="text-xs">
      <span className="inline-flex items-center gap-1">
        {nested && (
          <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="text-muted hover:text-ink">
            <ChevronRight aria-hidden="true" className={`size-3.5 transition-transform ${open ? "rotate-90" : ""}`} />
          </button>
        )}
        <span className="font-mono text-ink">{typeOf(schema) || "object"}</span>
        {Array.isArray(schema.enum) && <span className="font-mono text-muted">[{(schema.enum as unknown[]).map(String).join(", ")}]</span>}
      </span>
      {typeof schema.description === "string" && <p className="mt-0.5 text-muted">{schema.description}</p>}
      {nested && open && (
        <ul className="mt-1 ml-3 grid gap-1 border-l border-line pl-3">
          {names.map((n) => (
            <li key={n}>
              <span className="font-mono font-medium text-ink">{n}</span>
              {required.has(n) && <span className="ml-1 text-danger">*</span>}{" "}
              <SchemaTree schema={props[n] ?? null} labels={t} onRef={onRef} depth={depth + 1} />
            </li>
          ))}
          {items && (
            <li>
              <span className="font-mono text-muted">items</span> <SchemaTree schema={items} labels={t} onRef={onRef} depth={depth + 1} />
            </li>
          )}
          {variants.map((k) =>
            (schema[k] as ApiSchema[]).map((v, i) => (
              <li key={`${k}${i}`}>
                <span className="font-mono text-muted">{k}</span> <SchemaTree schema={v} labels={t} onRef={onRef} depth={depth + 1} />
              </li>
            )),
          )}
        </ul>
      )}
    </div>
  );
}
