"use client";

import { FolderInput, Trash } from "lucide-react";
import { useState } from "react";

import type { CatalogNode } from "@/lib/api";

import { Button } from "../ui/Button";
import { DangerZone } from "../places/DangerZone";
import { Panel } from "../ui/Panel";
import { DeleteDialog, EditForm, MoveDialog } from "./NodeForms";
import { UnlinkForgeButton } from "./UnlinkForgeDialog";
import { VisibilityPanel } from "./VisibilityPanel";
import type { CatalogLabels } from "./shared";

type Props = {
  node: CatalogNode;
  labels: CatalogLabels;
  fail: (e: unknown) => string;
  canWrite: boolean;
  canMove: boolean;
  canDelete: boolean;
  toTop: boolean;
  canSwitchVisibility: boolean;
  onSaved: () => void;
  onDeleted: () => void;
};

export function SettingsGeneral({ node, labels, fail, canWrite, canMove, canDelete, toTop, canSwitchVisibility, onSaved, onDeleted }: Props) {
  const t = labels.settings;
  const [dialog, setDialog] = useState<"move" | "delete" | null>(null);
  const canUnlink = canWrite && node.kind === "project" && !!node.managed;
  return (
    <div className="grid gap-6">
      <Panel title={t.general}>
        <EditForm key={node.updated_at} node={node} labels={labels} fail={fail} onSaved={onSaved} readOnly={!canWrite} />
      </Panel>
      {node.visibility && (
        <VisibilityPanel key={node.updated_at} node={node} labels={labels} fail={fail} canSwitch={canSwitchVisibility} onSaved={onSaved} />
      )}
      <DangerZone
        title={t.danger}
        rows={[
          ...(canUnlink ? [{ key: "unlink", text: labels.unlink.text, action: <UnlinkForgeButton node={node} labels={labels} fail={fail} onUnlinked={onSaved} /> }] : []),
          ...(canMove
            ? [
                {
                  key: "move",
                  text: t.moveText,
                  action: (
                    <Button onClick={() => setDialog("move")}>
                      <FolderInput aria-hidden="true" />
                      {labels.move.title}
                    </Button>
                  ),
                },
              ]
            : []),
          ...(canDelete
            ? [
                {
                  key: "delete",
                  text: t.deleteText,
                  action: (
                    <Button variant="danger" onClick={() => setDialog("delete")}>
                      <Trash aria-hidden="true" />
                      {labels.remove.button}
                    </Button>
                  ),
                },
              ]
            : []),
        ]}
      />
      {canMove && (
        <MoveDialog
          open={dialog === "move"}
          onOpenChange={(open) => setDialog(open ? "move" : null)}
          node={node}
          labels={labels}
          fail={fail}
          toTop={toTop}
          onMoved={onSaved}
        />
      )}
      {canDelete && (
        <DeleteDialog
          open={dialog === "delete"}
          onOpenChange={(open) => setDialog(open ? "delete" : null)}
          node={node}
          labels={labels}
          fail={fail}
          onDeleted={onDeleted}
        />
      )}
    </div>
  );
}
