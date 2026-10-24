"use client";

import { FolderInput, Trash } from "lucide-react";
import { useState } from "react";

import type { CatalogNode } from "@/lib/api";

import { Button } from "../ui/Button";
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
      {(canMove || canDelete || canUnlink) && (
        <Panel title={t.danger}>
          <div className="grid gap-4">
            {canUnlink && (
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-ink-2">{labels.unlink.text}</p>
                <UnlinkForgeButton node={node} labels={labels} fail={fail} onUnlinked={onSaved} />
              </div>
            )}
            {canMove && (
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-ink-2">{t.moveText}</p>
                <Button onClick={() => setDialog("move")}>
                  <FolderInput aria-hidden="true" />
                  {labels.move.title}
                </Button>
              </div>
            )}
            {canDelete && (
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-ink-2">{t.deleteText}</p>
                <Button variant="danger" onClick={() => setDialog("delete")}>
                  <Trash aria-hidden="true" />
                  {labels.remove.button}
                </Button>
              </div>
            )}
          </div>
        </Panel>
      )}
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
