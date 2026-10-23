"use client";

import { ChevronRight } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { filterOperations, groupByTag, operationAnchor, schemaAnchor, type ApiContent, type ApiDocument, type ApiOperation } from "@/lib/apis";

import { Badge } from "../ui/Badge";
import { Input } from "../ui/Field";
import { Table, Td, Th, Tr } from "../ui/Table";
import { ApiMarkdown, MethodBadge, SectionTitle } from "./ApiParts";
import type { ApiLabels } from "./ApiView";
import { SchemaTree } from "./SchemaTree";

type Props = { doc: ApiDocument; labels: ApiLabels };

function reveal(id: string) {
  window.history.replaceState(window.history.state, "", `${window.location.pathname}${window.location.search}#${id}`);
  requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ block: "start" }));
}

function ContentList({ content, labels: t, onRef }: { content: ApiContent[]; labels: ApiLabels; onRef: (n: string) => void }) {
  return (
    <ul className="grid gap-2">
      {content.map((c) => (
        <li key={c.media_type} className="grid gap-1">
          <span className="font-mono text-xs text-muted">{c.media_type}</span>
          <SchemaTree schema={c.schema} labels={t} onRef={onRef} />
        </li>
      ))}
    </ul>
  );
}

function OperationRow({ op, open, onToggle, labels: t, onRef }: { op: ApiOperation; open: boolean; onToggle: () => void; labels: ApiLabels; onRef: (n: string) => void }) {
  const places = t.places as Record<string, string>;
  return (
    <li id={operationAnchor(op)} className="scroll-mt-20 rounded-md border border-line bg-surface">
      <button type="button" aria-expanded={open} onClick={onToggle} className="flex w-full items-center gap-3 px-3 py-2 text-left hover:bg-surface-2">
        <ChevronRight aria-hidden="true" className={`size-4 shrink-0 text-muted transition-transform ${open ? "rotate-90" : ""}`} />
        <MethodBadge method={op.method} />
        <span className={`font-mono text-sm text-ink ${op.deprecated ? "line-through" : ""}`}>{op.path}</span>
        {op.summary && <span className="truncate text-sm text-muted">{op.summary}</span>}
        {op.deprecated && <Badge tone="amber">{t.deprecated}</Badge>}
      </button>
      {open && (
        <div className="grid gap-3 border-t border-line px-4 py-3">
          <ApiMarkdown text={op.description} />
          {op.parameters.length > 0 && (
            <div>
              <SectionTitle>{t.parameters}</SectionTitle>
              <Table>
                <thead>
                  <Tr>
                    <Th>{t.name}</Th>
                    <Th>{t.place}</Th>
                    <Th>{t.required}</Th>
                    <Th>{t.type}</Th>
                    <Th>{t.description}</Th>
                  </Tr>
                </thead>
                <tbody>
                  {op.parameters.map((p) => (
                    <Tr key={`${p.in}:${p.name}`}>
                      <Td className="font-mono">{p.name}</Td>
                      <Td>{places[p.in] ?? p.in}</Td>
                      <Td>{p.required ? t.yes : ""}</Td>
                      <Td>
                        <SchemaTree schema={p.schema} labels={t} onRef={onRef} />
                      </Td>
                      <Td>{p.description}</Td>
                    </Tr>
                  ))}
                </tbody>
              </Table>
            </div>
          )}
          {op.request_body && (
            <div>
              <SectionTitle>{t.requestBody}</SectionTitle>
              <ApiMarkdown text={op.request_body.description} />
              <ContentList content={op.request_body.content} labels={t} onRef={onRef} />
            </div>
          )}
          {op.responses.length > 0 && (
            <div>
              <SectionTitle>{t.responses}</SectionTitle>
              <ul className="grid gap-3">
                {op.responses.map((r) => (
                  <li key={r.status} className="grid gap-1">
                    <span className="text-sm">
                      <span className="font-mono font-semibold text-ink">{r.status}</span> <span className="text-muted">{r.description}</span>
                    </span>
                    <ContentList content={r.content} labels={t} onRef={onRef} />
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}
    </li>
  );
}

export function OpenApiView({ doc, labels: t }: Props) {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState<Set<string>>(() => {
    const id = typeof window === "undefined" ? "" : decodeURIComponent(window.location.hash.slice(1));
    return new Set(id ? [id] : []);
  });
  const ops = useMemo(() => doc.operations ?? [], [doc.operations]);
  const shown = useMemo(() => filterOperations(ops, query), [ops, query]);
  const groups = useMemo(() => groupByTag(doc.tags ?? [], shown), [doc.tags, shown]);

  const openRef = (name: string) => {
    setOpen((s) => new Set(s).add(schemaAnchor(name)));
    reveal(schemaAnchor(name));
  };
  const toggle = (id: string) =>
    setOpen((s) => {
      const next = new Set(s);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  useEffect(() => {
    const id = decodeURIComponent(window.location.hash.slice(1));
    if (!id) return;
    requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView({ block: "start" }));
  }, []);

  return (
    <div className="grid gap-2">
      <header className="grid gap-1">
        <h2 className="text-lg font-semibold text-ink">
          {doc.info?.title} <span className="font-mono text-sm text-muted">{doc.info?.version}</span>
        </h2>
        <p className="text-xs text-muted">
          {doc.format} {doc.spec_version}
        </p>
        <ApiMarkdown text={doc.info?.description} />
        {(doc.servers ?? []).length > 0 && (
          <ul className="text-sm">
            {doc.servers!.map((s) => (
              <li key={s.url} className="font-mono text-ink-2">
                {s.url}
              </li>
            ))}
          </ul>
        )}
      </header>
      <SectionTitle>{t.operations}</SectionTitle>
      <Input aria-label={t.filter} placeholder={t.filter} value={query} onChange={(e) => setQuery(e.target.value)} className="max-w-sm" />
      {shown.length === 0 && ops.length > 0 && <p className="text-sm text-muted">{t.noOperations}</p>}
      {groups.map((g) => (
        <section key={g.tag ?? ""} className="grid gap-2">
          <h4 className="mt-3 text-sm font-semibold text-ink">{g.tag ?? t.untagged}</h4>
          <ul className="grid gap-2">
            {g.operations.map((op) => (
              <OperationRow key={op.key} op={op} open={open.has(operationAnchor(op))} onToggle={() => toggle(operationAnchor(op))} labels={t} onRef={openRef} />
            ))}
          </ul>
        </section>
      ))}
      {(doc.schemas ?? []).length > 0 && (
        <>
          <SectionTitle>{t.schemas}</SectionTitle>
          <ul className="grid gap-2">
            {doc.schemas!.map((s) => (
              <li key={s.name} id={schemaAnchor(s.name)} className="scroll-mt-20 rounded-md border border-line bg-surface">
                <button
                  type="button"
                  aria-expanded={open.has(schemaAnchor(s.name))}
                  onClick={() => toggle(schemaAnchor(s.name))}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left font-mono text-sm text-ink hover:bg-surface-2"
                >
                  <ChevronRight aria-hidden="true" className={`size-4 text-muted transition-transform ${open.has(schemaAnchor(s.name)) ? "rotate-90" : ""}`} />
                  {s.name}
                </button>
                {open.has(schemaAnchor(s.name)) && (
                  <div className="border-t border-line px-4 py-3">
                    <SchemaTree schema={s.schema} labels={t} onRef={openRef} />
                  </div>
                )}
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
