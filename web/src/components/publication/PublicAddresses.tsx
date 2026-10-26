"use client";

import { ExternalLink, Globe } from "lucide-react";
import { useEffect, useState } from "react";

import type { CatalogNode } from "@/lib/api";
import { apiGet } from "@/lib/api";
import { useCatalogVersion } from "@/lib/catalogEvents";
import type { Publication } from "@/lib/publication";

import { Panel } from "../ui/Panel";
import type { CatalogLabels } from "../catalog/shared";

export function PublicAddresses({ node, labels }: { node: CatalogNode; labels: CatalogLabels }) {
  const [entries, setEntries] = useState<Publication["entries"]>([]);
  const version = useCatalogVersion();

  useEffect(() => {
    let live = true;
    apiGet<Publication>(`/v1/catalog/nodes/${node.id}/publication`)
      .then((p) => live && setEntries(p.entries))
      .catch(() => live && setEntries([]));
    return () => {
      live = false;
    };
  }, [node.id, version]);

  if (entries.length === 0) return null;
  return (
    <Panel title={labels.publication.addresses} icon={Globe}>
      <ul className="grid gap-1 text-sm">
        {entries.map((e) => (
          <li key={e.url}>
            <a href={e.url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 font-mono break-all text-signal hover:underline">
              {e.url}
              <ExternalLink aria-hidden="true" className="size-3 shrink-0" />
            </a>
          </li>
        ))}
      </ul>
    </Panel>
  );
}
