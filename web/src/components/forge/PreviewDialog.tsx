"use client";

import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import { apiSend, type Items, type PreviewItem } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Message } from "../ui/Message";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { ForgeLabels } from "./shared";

const actionTone: Record<PreviewItem["action"], Tone> = { create: "signal", update: "neutral", move: "amber", orphan: "danger", skip: "outline" };

type Props = { url: string; labels: ForgeLabels; fail: (e: unknown) => string; onClose: () => void };

export function PreviewDialog({ url, labels: t, fail, onClose }: Props) {
  const { common, errors } = useUiText();
  const [items, setItems] = useState<PreviewItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiSend<Items<PreviewItem>>("POST", url as `/${string}`)
      .then((r) => setItems(r.items))
      .catch((e: unknown) => setError(fail(e)));
  }, [url, fail]);

  const reason = (code: string) => errorText(errors, code);

  return (
    <Dialog open size="lg" onOpenChange={(open) => !open && onClose()} title={t.previewTitle} footer={<Button onClick={onClose}>{t.close}</Button>}>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!items && !error && <SkeletonTable rows={4} label={common.loading} />}
      {items && items.length === 0 && <p className="text-sm text-muted">{t.previewEmpty}</p>}
      {items && items.length > 0 && (
        <div className="max-h-[60vh] overflow-y-auto">
          <Table>
            <thead>
              <tr>
                <Th>{t.owner}</Th>
                <Th>{t.problems}</Th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <Tr key={it.full_path}>
                  <Td>
                    <code className="text-ink">{it.full_path}</code>
                  </Td>
                  <Td>
                    <Badge tone={actionTone[it.action]}>{t.actions[it.action]}</Badge>
                    {it.code && <span className="ml-2 text-xs text-muted">{reason(it.code)}</span>}
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        </div>
      )}
    </Dialog>
  );
}
