"use client";

import { useState } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiSend, errorCode } from "@/lib/api";
import { normalizeRights, RIGHT_ACTIONS, RIGHT_SECTIONS, SECTION_ACTIONS, type RightAction, type RightSection, type Rights } from "@/lib/rights";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Table, Td, Th, Tr } from "../ui/Table";

type Labels = Messages["admin"]["groups"]["permissions"];

type Props = { groupId: string; initial: Rights; editable: boolean; labels: Labels; onSaved: () => void };

export function GroupPermissions({ groupId, initial, editable, labels: t, onSaved }: Props) {
  const { errors } = useUiText();
  const [rights, setRights] = useState<Rights>(initial);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  const toggle = (section: RightSection, action: RightAction, on: boolean) => {
    setRights((r) => {
      const held = new Set(r[section] ?? []);
      if (on) held.add(action);
      else held.delete(action);
      return { ...r, [section]: [...held] };
    });
  };

  const save = async () => {
    setBusy(true);
    try {
      const saved = await apiSend<{ permissions: Rights }>("PUT", `/v1/groups/${groupId}/permissions`, { permissions: normalizeRights(rights) });
      setRights(saved.permissions);
      setNote(null);
      toast.success(t.saved);
      onSaved();
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Panel title={t.title}>
      <p className="mb-3 text-sm text-ink-2">{t.lead}</p>
      <Table>
        <thead>
          <tr>
            <Th>{t.section}</Th>
            {RIGHT_ACTIONS.map((a) => (
              <Th key={a} className="text-center">
                {t.actions[a]}
              </Th>
            ))}
          </tr>
        </thead>
        <tbody>
          {RIGHT_SECTIONS.map((section) => (
            <Tr key={section}>
              <Td className="font-medium text-ink">{t.sections[section]}</Td>
              {RIGHT_ACTIONS.map((action) => (
                <Td key={action} className="text-center">
                  {SECTION_ACTIONS[section].includes(action) && (
                    <input
                      type="checkbox"
                      className="size-4 rounded-sm border-field accent-[var(--signal)]"
                      aria-label={`${t.sections[section]}: ${t.actions[action]}`}
                      checked={(rights[section] ?? []).includes(action)}
                      disabled={!editable || busy}
                      onChange={(e) => toggle(section, action, e.target.checked)}
                    />
                  )}
                </Td>
              ))}
            </Tr>
          ))}
        </tbody>
      </Table>
      <Message note={note} className="mt-3" />
      {editable && (
        <Button variant="primary" busy={busy} className="mt-3" onClick={() => void save()}>
          {t.save}
        </Button>
      )}
    </Panel>
  );
}
