"use client";

import { TriangleAlert } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "../ui/Button";
import { CopyButton } from "../ui/CopyButton";
import { Dialog } from "../ui/Dialog";

type Props = {
  secret: string;
  labels: { title: string; warning: string; copy: string; close: string };
  onClose: () => void;
  children?: ReactNode;
};

export function KeyDialog({ secret, labels, onClose, children }: Props) {
  return (
    <Dialog
      open
      persistent
      onOpenChange={(open) => !open && onClose()}
      title={labels.title}
      footer={
        <Button variant="primary" onClick={onClose}>
          {labels.close}
        </Button>
      }
    >
      <p role="alert" className="mb-4 flex items-start gap-2.5 rounded-md bg-amber-soft px-3 py-2.5 text-sm text-amber">
        <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
        {labels.warning}
      </p>
      <div className="flex flex-wrap items-center gap-2 rounded-lg border border-line bg-surface-2 p-2 pl-3">
        <code className="min-w-0 flex-1 font-mono text-sm break-all text-ink select-all">{secret}</code>
        <CopyButton text={secret} label={labels.copy} variant="primary" />
      </div>
      {children}
    </Dialog>
  );
}
