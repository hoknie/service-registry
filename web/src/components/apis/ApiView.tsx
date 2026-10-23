"use client";

import type { ApiDocument, ApiView as View } from "@/lib/apis";
import { structuredKind } from "@/lib/structured";

import type { CatalogLabels } from "../catalog/shared";
import { StructuredView } from "../knowledge/StructuredView";
import { Segmented } from "../ui/Segmented";
import { ApiProblems } from "./ApiProblems";
import { OpenApiView } from "./OpenApiView";
import { ProtoView } from "./ProtoView";

export type ApiLabels = CatalogLabels["docs"]["api"];

type Props = {
  doc: ApiDocument;
  content: string | null;
  view: View;
  onView: (view: View) => void;
  labels: CatalogLabels["docs"];
  resolveImport: (imp: string) => string | null;
  fileHref: (path: string) => string;
  onOpenFile: (path: string) => void;
};

const emptyModel = (doc: ApiDocument) => doc.problems.some((p) => p.code === "api.file_skipped" || p.code === "api.invalid_document") && (doc.operations ?? doc.messages ?? []).length === 0 && (doc.services ?? []).length === 0;

export function ApiView({ doc, content, view, onView, labels, resolveImport, fileHref, onOpenFile }: Props) {
  const t = labels.api;
  const tree = content !== null && structuredKind(doc.path) !== null;
  const options = [{ value: "api", label: t.view }];
  if (tree) options.push({ value: "tree", label: t.tree });
  if (content !== null) options.push({ value: "source", label: t.source });
  const current = view === "tree" && !tree ? "api" : view === "source" && content === null ? "api" : view;
  return (
    <div className="grid gap-3">
      {options.length > 1 && <Segmented label={t.views} value={current} options={options} onChange={(v) => onView(v as View)} className="justify-self-start" />}
      {current === "tree" && content !== null && <StructuredView kind={structuredKind(doc.path)!} content={content} labels={labels.viewer} />}
      {current === "source" && content !== null && (
        <pre className="overflow-x-auto rounded-md bg-surface-2 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink-2">{content}</pre>
      )}
      {current === "api" && (
        <>
          <ApiProblems problems={doc.problems} labels={t} />
          {!emptyModel(doc) && doc.format !== "proto" && <OpenApiView doc={doc} labels={t} />}
          {!emptyModel(doc) && doc.format === "proto" && (
            <ProtoView doc={doc} labels={t} resolveImport={resolveImport} fileHref={fileHref} onOpenFile={onOpenFile} />
          )}
        </>
      )}
    </div>
  );
}
