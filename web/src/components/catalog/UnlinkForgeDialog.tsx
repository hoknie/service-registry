"use client";

import { Unlink } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { apiGet, apiSend, type CatalogNode, type KnowledgeSource } from "@/lib/api";

import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import type { CatalogLabels } from "./shared";

type Props = {
  node: CatalogNode;
  labels: CatalogLabels;
  fail: (e: unknown) => string;
  onUnlinked: () => void;
  size?: "sm" | "md";
};

export function UnlinkForgeButton({ node, labels, fail, onUnlinked, size }: Props) {
  const t = labels.unlink;
  const [open, setOpen] = useState(false);
  const [noSecret, setNoSecret] = useState(false);

  useEffect(() => {
    if (!open) return;
    let live = true;
    apiGet<KnowledgeSource | null>(`/v1/catalog/nodes/${node.id}/knowledge/source`)
      .then((s) => live && setNoSecret(!!s && s.origin === "forge" && s.credentials.mode !== "secret"))
      .catch(() => live && setNoSecret(false));
    return () => {
      live = false;
    };
  }, [open, node.id]);

  return (
    <>
      <Button size={size} onClick={() => setOpen(true)}>
        <Unlink aria-hidden="true" />
        {t.button}
      </Button>
      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={t.title}
        text={noSecret ? `${t.confirm} ${t.noSecret}` : t.confirm}
        confirm={t.submit}
        onConfirm={async () => {
          try {
            await apiSend("POST", `/v1/catalog/nodes/${node.id}/forge/unlink`);
            toast.success(t.done);
            onUnlinked();
            return null;
          } catch (e) {
            return fail(e);
          }
        }}
      />
    </>
  );
}
