"use client";

import type { PickedNode, TokenAccess } from "@/lib/tokenNodes";

import { Segmented } from "../ui/Segmented";
import { TokenNodes, type NodeScopeLabels } from "./TokenNodes";

type Props = {
  access: TokenAccess;
  onAccess: (access: TokenAccess) => void;
  nodes: PickedNode[];
  onNodes: (nodes: PickedNode[]) => void;
  labels: NodeScopeLabels;
  note?: string;
};

export function NodeScopePicker({ access, onAccess, nodes, onNodes, labels: t, note }: Props) {
  return (
    <fieldset className="grid gap-2">
      <legend className="mb-1.5 text-sm font-medium text-ink-2">{t.access}</legend>
      <Segmented
        label={t.access}
        value={access}
        onChange={(v) => onAccess(v as TokenAccess)}
        options={[
          { value: "all", label: t.accessAll },
          { value: "nodes", label: t.accessNodes },
        ]}
        className="justify-self-start"
      />
      {access === "nodes" && (
        <>
          {note && <p className="text-xs text-muted">{note}</p>}
          <TokenNodes value={nodes} onChange={onNodes} labels={t} />
        </>
      )}
    </fieldset>
  );
}
