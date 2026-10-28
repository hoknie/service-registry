"use client";

import { Power, PowerOff } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { isOff, type SkillFolder, type SkillRef, type SkillStatus } from "@/lib/skills";
import { patchFolder } from "@/lib/skillsApi";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Panel } from "../ui/Panel";
import { Tooltip } from "../ui/Tooltip";

type Labels = Messages["skills"]["off"];

export function DisabledBadge({ item, labels: t }: { item: { status?: SkillStatus; disabled?: boolean; disabled_by: SkillRef | null }; labels: Labels }) {
  if (!isOff(item)) return null;
  const tip = item.disabled_by ? format(t.byFolder, { name: item.disabled_by.name }) : t.badge;
  return (
    <Tooltip content={tip}>
      <Badge tone="danger">
        <PowerOff aria-hidden="true" className="size-3" /> {t.badge}
      </Badge>
    </Tooltip>
  );
}

export function FolderPower({ folder, labels: t, onChanged }: { folder: SkillFolder; labels: Labels; onChanged: () => void }) {
  const { errors } = useUiText();
  const [busy, setBusy] = useState(false);
  const toggle = async () => {
    setBusy(true);
    try {
      await patchFolder(folder.id, { disabled: !folder.disabled });
      onChanged();
    } catch (e) {
      toast.error(errorText(errors, errorCode(e)));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Panel title={t.folderTitle} description={t.folderText}>
      <div className="flex flex-wrap items-center gap-3 text-sm">
        {folder.disabled_by && <span className="text-muted">{format(t.folderOffBy, { name: folder.disabled_by.name })}</span>}
        <Button busy={busy} variant={folder.disabled ? "primary" : "secondary"} onClick={toggle}>
          {folder.disabled ? <Power aria-hidden="true" /> : <PowerOff aria-hidden="true" />}
          {folder.disabled ? t.folderEnable : t.folderDisable}
        </Button>
      </div>
    </Panel>
  );
}
