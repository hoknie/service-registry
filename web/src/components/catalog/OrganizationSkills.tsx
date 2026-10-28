"use client";

import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type CatalogNode } from "@/lib/api";
import { SKILLS_BASE, type SkillSpace } from "@/lib/skills";

import { VersionPolicyForm } from "../skills/VersionPolicyForm";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { MarkdownField } from "../places/MarkdownField";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";

export function OrganizationSkills({
  node,
  labels: t,
}: {
  node: Pick<CatalogNode, "id">;
  labels: Messages["skills"];
}) {
  const o = t.organization;
  const { errors } = useUiText();
  const path = `${SKILLS_BASE}/organizations/${node.id}` as const;
  const [space, setSpace] = useState<SkillSpace | null>(null);
  const [description, setDescription] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    apiGet<SkillSpace>(path)
      .then((s) => {
        setSpace(s);
        setDescription(s.description);
      })
      .catch((e: unknown) =>
        setNote({ kind: "error", text: errorText(errors, errorCode(e)) }),
      );
  }, [path, errors]);

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      const s = await apiSend<SkillSpace>("PUT", path, { description });
      setSpace(s);
      setDescription(s.description);
      setNote({ kind: "success", text: o.saved });
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="grid gap-4">
      <Panel title={o.title} description={o.lead}>
        <div className="grid gap-4">
          <MarkdownField
            label={o.description}
            hint={o.descriptionHint}
            rows={4}
            maxLength={2000}
            value={description}
            readOnly={!space?.can_write}
            onChange={setDescription}
          />
          <Message note={note} />
          <div className="flex flex-wrap items-center gap-3">
            {space?.can_write && (
              <Button
                variant="primary"
                busy={busy}
                onClick={save}
                disabled={description === space.description}
              >
                {o.save}
              </Button>
            )}
          </div>
        </div>
      </Panel>
      <VersionPolicyForm path={`${path}/version-policy`} labels={t} />
    </div>
  );
}
