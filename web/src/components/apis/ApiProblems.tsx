"use client";

import { ChevronRight } from "lucide-react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { ApiProblem } from "@/lib/apis";

import { useUiText } from "../UiText";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../ui/Collapsible";
import type { ApiLabels } from "./ApiView";

export function ApiProblems({ problems, labels: t }: { problems: ApiProblem[]; labels: ApiLabels }) {
  const { errors } = useUiText();
  if (problems.length === 0) return null;
  return (
    <Collapsible className="mb-4 rounded-md border border-line bg-amber-soft px-3 py-2 text-sm">
      <CollapsibleTrigger className="group flex items-center gap-1.5 font-medium text-ink">
        <ChevronRight aria-hidden="true" className="size-4 transition-transform group-data-[state=open]:rotate-90" />
        {format(t.problems, { count: String(problems.length) })}
      </CollapsibleTrigger>
      <CollapsibleContent>
        <ul className="mt-2 grid gap-1 text-ink-2">
          {problems.map((p, i) => (
            <li key={i}>
              {errorText(errors, p.code)} <span className="font-mono text-xs">{p.path}{p.line !== null ? `:${p.line}` : ""}</span>
              {p.code === "api.import_not_found" || p.code === "api.ref_unresolved" ? (
                <span className="ml-1 font-mono text-xs text-muted">{p.message}</span>
              ) : null}
            </li>
          ))}
        </ul>
      </CollapsibleContent>
    </Collapsible>
  );
}
