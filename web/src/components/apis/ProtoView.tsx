"use client";

import { useEffect, useMemo, useState } from "react";

import { format } from "@/i18n/format";
import { importedType, localType, protoAnchor, rememberAnchor, takeAnchor, type ApiDocument } from "@/lib/apis";

import { Badge } from "../ui/Badge";
import { Input } from "../ui/Field";
import { Table, Td, Th, Tr } from "../ui/Table";
import { MethodBadge, SectionTitle } from "./ApiParts";
import type { ApiLabels } from "./ApiView";

type Props = { doc: ApiDocument; labels: ApiLabels; resolveImport: (imp: string) => string | null; fileHref: (path: string) => string; onOpenFile: (path: string) => void };

export function ProtoView({ doc, labels: t, resolveImport, fileHref, onOpenFile }: Props) {
  const [query, setQuery] = useState("");
  const known = useMemo(() => new Set([...(doc.messages ?? []).map((m) => m.full_name), ...(doc.enums ?? []).map((e) => e.full_name)]), [doc.messages, doc.enums]);
  const q = query.trim().toLowerCase();
  const match = (name: string) => !q || name.toLowerCase().includes(q);

  useEffect(() => {
    const anchor = takeAnchor(window.location.hash);
    if (anchor) document.getElementById(anchor)?.scrollIntoView();
  }, [doc]);

  const typeLink = (type: string) => {
    const local = localType(type, doc.package, known);
    if (local) {
      return (
        <a href={`#${protoAnchor(local)}`} className="font-mono text-signal hover:underline">
          {type}
        </a>
      );
    }
    const imported = importedType(type, doc.type_files);
    if (imported) {
      const anchor = protoAnchor(imported.name);
      return (
        <a
          href={`${fileHref(imported.path)}#${anchor}`}
          title={format(t.definedIn, { path: imported.path })}
          className="font-mono text-signal hover:underline"
          onClick={(e) => {
            if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
            e.preventDefault();
            rememberAnchor(anchor);
            onOpenFile(imported.path);
          }}
        >
          {type}
        </a>
      );
    }
    return <span className="font-mono text-ink-2">{type}</span>;
  };

  const services = (doc.services ?? [])
    .map((s) => ({ ...s, methods: match(s.name) ? s.methods : s.methods.filter((m) => match(m.name)) }))
    .filter((s) => match(s.name) || s.methods.length > 0);
  const messages = (doc.messages ?? []).filter((m) => match(m.full_name));
  const enums = (doc.enums ?? []).filter((e) => match(e.full_name));

  return (
    <div className="grid gap-2 text-sm">
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
        <dt className="text-muted">{t.package}</dt>
        <dd className="font-mono text-ink">{doc.package ?? "—"}</dd>
        <dt className="text-muted">{t.syntax}</dt>
        <dd className="font-mono text-ink">{doc.spec_version}</dd>
        {(doc.imports ?? []).length > 0 && (
          <>
            <dt className="text-muted">{t.imports}</dt>
            <dd>
              <ul>
                {doc.imports!.map((imp) => {
                  const target = resolveImport(imp);
                  return (
                    <li key={imp} className="font-mono text-xs">
                      {target ? (
                        <a
                          href={fileHref(target)}
                          className="text-signal hover:underline"
                          onClick={(e) => {
                            e.preventDefault();
                            onOpenFile(target);
                          }}
                        >
                          {imp}
                        </a>
                      ) : (
                        <span className="text-ink-2">{imp}</span>
                      )}
                    </li>
                  );
                })}
              </ul>
            </dd>
          </>
        )}
      </dl>
      <Input aria-label={t.filter} placeholder={t.filter} value={query} onChange={(e) => setQuery(e.target.value)} className="max-w-sm" />
      {services.length > 0 && <SectionTitle>{t.services}</SectionTitle>}
      {services.map((s) => (
        <section key={s.full_name} id={protoAnchor(s.full_name)} className="scroll-mt-20 rounded-md border border-line bg-surface p-3">
          <h4 className="font-mono font-semibold text-ink">{s.name}</h4>
          {s.description && <p className="mt-1 whitespace-pre-wrap text-muted">{s.description}</p>}
          <ul className="mt-2 grid gap-2">
            {s.methods.map((m) => (
              <li key={m.name} className="grid gap-0.5">
                <span className={`flex flex-wrap items-center gap-1.5 font-mono ${m.deprecated ? "line-through" : ""}`}>
                  <span className="font-semibold text-ink">{m.name}</span>(
                  {m.client_streaming && <Badge tone="cobalt">{t.stream}</Badge>}
                  {typeLink(m.input)}) → {m.server_streaming && <Badge tone="cobalt">{t.stream}</Badge>}
                  {typeLink(m.output)}
                </span>
                {m.http && (
                  <span className="flex items-center gap-2">
                    <MethodBadge method={m.http.method} />
                    <span className="font-mono text-xs text-ink-2">{m.http.path}</span>
                  </span>
                )}
                {m.description && <span className="whitespace-pre-wrap text-muted">{m.description}</span>}
              </li>
            ))}
          </ul>
        </section>
      ))}
      {messages.length > 0 && <SectionTitle>{t.messages}</SectionTitle>}
      {messages.map((m) => (
        <section key={m.full_name} id={protoAnchor(m.full_name)} className="scroll-mt-20 grid gap-2">
          <h4 className="font-mono font-semibold text-ink">
            {m.full_name} {m.deprecated && <Badge tone="amber">{t.deprecated}</Badge>}
          </h4>
          {m.description && <p className="whitespace-pre-wrap text-muted">{m.description}</p>}
          {m.fields.length > 0 && (
            <Table>
              <thead>
                <Tr>
                  <Th numeric>{t.number}</Th>
                  <Th>{t.name}</Th>
                  <Th>{t.type}</Th>
                  <Th>{t.label}</Th>
                  <Th>{t.description}</Th>
                </Tr>
              </thead>
              <tbody>
                {m.fields.map((f) => (
                  <Tr key={f.number}>
                    <Td numeric>{f.number}</Td>
                    <Td className={`font-mono ${f.deprecated ? "line-through" : ""}`}>{f.name}</Td>
                    <Td>
                      {f.map ? (
                        <span className="font-mono">
                          map&lt;{typeLink(f.map.key)}, {typeLink(f.map.value)}&gt;
                        </span>
                      ) : (
                        typeLink(f.type)
                      )}
                    </Td>
                    <Td className="font-mono text-muted">{[f.label, f.oneof ? `oneof ${f.oneof}` : null].filter(Boolean).join(" · ")}</Td>
                    <Td className="whitespace-pre-wrap">{f.description}</Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          )}
        </section>
      ))}
      {enums.length > 0 && <SectionTitle>{t.enums}</SectionTitle>}
      {enums.map((e) => (
        <section key={e.full_name} id={protoAnchor(e.full_name)} className="scroll-mt-20 grid gap-1">
          <h4 className="font-mono font-semibold text-ink">{e.full_name}</h4>
          {e.description && <p className="whitespace-pre-wrap text-muted">{e.description}</p>}
          <ul className="grid gap-0.5 font-mono text-xs">
            {e.values.map((v) => (
              <li key={v.name}>
                {v.name} = {v.number}
                {v.description && <span className="ml-2 font-sans text-muted">{v.description}</span>}
              </li>
            ))}
          </ul>
        </section>
      ))}
      {services.length === 0 && messages.length === 0 && enums.length === 0 && <p className="text-muted">{t.empty}</p>}
    </div>
  );
}
