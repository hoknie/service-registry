"use client";

import { Pencil, UsersRound } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { errorCode, type OwnGroup } from "@/lib/api";
import { ownGroups, putOwnGroupContacts } from "@/lib/ownersApi";

import { useUiText } from "../UiText";
import { GroupContacts } from "../admin/GroupContacts";
import { GroupContactsView } from "../owners/GroupContactsView";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonRows } from "../ui/Skeleton";

type Props = { labels: Messages["account"]["groups"]; contactLabels: Messages["admin"]["groups"]["contacts"] };

export function GroupsTab({ labels, contactLabels }: Props) {
  const { errors, common } = useUiText();
  const [groups, setGroups] = useState<OwnGroup[] | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [editing, setEditing] = useState<OwnGroup | null>(null);

  const load = useCallback(async () => {
    try {
      setGroups(await ownGroups());
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    }
  }, [errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  return (
    <Panel title={labels.title} description={labels.lead}>
      {!groups ? (
        note ? <Message note={note} /> : <SkeletonRows rows={3} label={common.loading} />
      ) : groups.length === 0 ? (
        <EmptyState icon={UsersRound} title={labels.empty} />
      ) : (
        <ul className="divide-y divide-line rounded-lg border border-line">
          {groups.map((g) => (
            <li key={g.id} className="grid gap-2 px-3 py-3">
              <span className="flex flex-wrap items-center gap-2">
                <UsersRound aria-hidden="true" className="size-4 shrink-0 text-muted" />
                <span className="min-w-0 flex-1 truncate font-medium text-ink">{g.name}</span>
                {g.can_edit_contacts && (
                  <Button size="sm" onClick={() => setEditing(g)}>
                    <Pencil aria-hidden="true" />
                    {labels.edit}
                  </Button>
                )}
              </span>
              {g.contacts.length === 0 && !g.on_call_url ? (
                <p className="text-sm text-muted">{labels.noContacts}</p>
              ) : (
                <GroupContactsView contacts={g.contacts} onCallUrl={g.on_call_url} labels={labels} />
              )}
            </li>
          ))}
        </ul>
      )}
      <Dialog open={editing !== null} onOpenChange={(open) => !open && setEditing(null)} title={editing?.name ?? ""} description={contactLabels.lead} size="lg">
        {editing && (
          <GroupContacts
            group={editing}
            labels={contactLabels}
            plain
            save={putOwnGroupContacts}
            onSaved={() => {
              setEditing(null);
              void load();
            }}
          />
        )}
      </Dialog>
    </Panel>
  );
}
