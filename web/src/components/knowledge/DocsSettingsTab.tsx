"use client";

import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { apiGet, type CatalogNode, type Knowledge } from "@/lib/api";

import type { CatalogLabels } from "../catalog/shared";
import { KnowledgeSettings } from "./KnowledgeSettings";
import { SourcePanel } from "./SourcePanel";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; canWrite: boolean; onChanged?: () => void };

export function DocsSettingsTab({ node, locale, labels, canWrite, onChanged }: Props) {
  const project = node.kind === "project";
  const [synced, setSynced] = useState(false);

  useEffect(() => {
    if (!project) return;
    let live = true;
    apiGet<Knowledge>(`/v1/catalog/nodes/${node.id}/knowledge`)
      .then((k) => live && setSynced(k.synced))
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, [project, node.id]);

  return (
    <div className="grid gap-6">
      {project && <SourcePanel projectId={node.id} parentId={node.parent_id} canAccess={!!node.permissions?.includes("catalog.access")} synced={synced} canWrite={canWrite} labels={labels.docs.source} onChanged={() => onChanged?.()} />}
      <KnowledgeSettings
        key={node.id}
        nodeId={node.id}
        project={project}
        canWrite={canWrite}
        locale={locale}
        labels={labels.docs.settings}
        onSaved={onChanged}
      />
    </div>
  );
}
