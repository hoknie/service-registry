"use client";

import { Check, GitBranch } from "lucide-react";
import { useEffect, useState } from "react";

import { apiGet, type Branch, type Page } from "@/lib/api";

import { Button } from "../ui/Button";
import { Input } from "../ui/Field";
import { Menu, MenuContent, MenuItem, MenuLabel, MenuTrigger } from "../ui/Menu";
import type { CatalogLabels } from "./shared";

type Props = {
  projectId: string;
  current: string | null;
  labels: CatalogLabels["branches"];
  onSelect: (name: string) => void;
};

export function BranchPicker({ projectId, current, labels: t, onSelect }: Props) {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const [items, setItems] = useState<Branch[]>([]);

  useEffect(() => {
    if (!open) return;
    const query = new URLSearchParams({ state: "all", limit: "20" });
    if (q) query.set("q", q);
    const timer = window.setTimeout(() => {
      apiGet<Page<Branch>>(`/v1/catalog/nodes/${projectId}/branches?${query}`)
        .then((page) => setItems(page.items))
        .catch(() => setItems([]));
    }, 150);
    return () => window.clearTimeout(timer);
  }, [open, q, projectId]);

  return (
    <Menu open={open} onOpenChange={setOpen}>
      <MenuTrigger asChild>
        <Button size="sm" aria-label={t.picker}>
          <GitBranch aria-hidden="true" />
          <span className="max-w-48 truncate font-mono">{current ?? t.picker}</span>
        </Button>
      </MenuTrigger>
      <MenuContent align="start" className="w-72">
        <MenuLabel>{t.picker}</MenuLabel>
        <div className="px-2 pb-2">
          <Input
            autoFocus
            aria-label={t.pickerSearch}
            placeholder={t.pickerSearch}
            spellCheck={false}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => e.stopPropagation()}
          />
        </div>
        <div className="max-h-72 overflow-y-auto">
          {items.length === 0 && <p className="px-3 py-2 text-sm text-muted">{t.pickerEmpty}</p>}
          {items.map((b) => (
            <MenuItem key={b.name} onSelect={() => onSelect(b.name)}>
              <Check aria-hidden="true" className={b.name === current ? "opacity-100" : "opacity-0"} />
              <span className="min-w-0 flex-1 truncate font-mono text-sm">{b.name}</span>
              {b.is_default && <span className="text-xs text-muted">{t.badges.default}</span>}
            </MenuItem>
          ))}
        </div>
      </MenuContent>
    </Menu>
  );
}
