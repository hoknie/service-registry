"use client";

import { UserMinus, UsersRound } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode, type ManagedGroup, type Member } from "@/lib/api";
import { managedGroup, managedGroups, setMember } from "@/lib/managedGroups";

import { catalogHref } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { MemberPicker } from "./MemberPicker";

type Labels = Messages["account"]["groups"];

export function ManagedGroups({ locale, labels }: { locale: Locale; labels: Labels }) {
  const { errors } = useUiText();
  const [groups, setGroups] = useState<ManagedGroup[]>([]);
  const [note, setNote] = useState<Note>(null);

  useEffect(() => {
    managedGroups()
      .then(setGroups)
      .catch((e: unknown) => setNote({ kind: "error", text: errorText(errors, errorCode(e)) }));
  }, [errors]);

  if (groups.length === 0) return note ? <Message note={note} /> : null;
  return (
    <Panel title={labels.managedTitle} description={labels.managedLead}>
      <ul className="grid gap-4">
        {groups.map((g) => (
          <ManagedGroupItem key={g.id} locale={locale} group={g} labels={labels} />
        ))}
      </ul>
    </Panel>
  );
}

function ManagedGroupItem({ locale, group, labels }: { locale: Locale; group: ManagedGroup; labels: Labels }) {
  const { errors } = useUiText();
  const [members, setMembers] = useState<Member[] | null>(null);
  const [note, setNote] = useState<Note>(null);
  const fail = useCallback((e: unknown) => setNote({ kind: "error", text: errorText(errors, errorCode(e)) }), [errors]);

  const load = useCallback(async () => {
    try {
      setMembers((await managedGroup(group.id)).members);
      setNote(null);
    } catch (e) {
      fail(e);
    }
  }, [group.id, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const change = (userId: string, member: boolean) =>
    void setMember(group.id, userId, member)
      .then(load)
      .catch(fail);

  return (
    <li className="grid gap-3 rounded-lg border border-line p-3">
      <span className="flex flex-wrap items-center gap-2">
        <UsersRound aria-hidden="true" className="size-4 shrink-0 text-muted" />
        <span className="font-medium text-ink">{group.name}</span>
        <Badge>{members?.length ?? group.member_count}</Badge>
      </span>
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted">
        <span>{labels.managedNodes}:</span>
        {group.nodes.map((n) => (
          <Link key={n.id} href={catalogHref(locale, n.id)} className="font-mono text-xs text-signal hover:underline">
            {n.path ?? n.name ?? n.id}
          </Link>
        ))}
      </p>
      {members && members.length === 0 && <p className="text-sm text-muted">{labels.noMembers}</p>}
      {members && members.length > 0 && (
        <ul aria-label={labels.members} className="divide-y divide-line rounded-md border border-line">
          {members.map((m) => (
            <li key={m.id} className="flex items-center gap-3 px-3 py-2">
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-ink">{m.display_name}</span>
                <span className="block truncate text-xs text-muted">{m.email}</span>
              </span>
              {m.source.startsWith("oauth:") ? (
                <Badge tone="cobalt">{format(labels.fromProvider, { provider: m.source.slice("oauth:".length) })}</Badge>
              ) : (
                <Button size="sm" variant="ghost" aria-label={`${labels.remove}: ${m.display_name}`} onClick={() => change(m.id, false)}>
                  <UserMinus aria-hidden="true" />
                  <span className="hidden sm:inline">{labels.remove}</span>
                </Button>
              )}
            </li>
          ))}
        </ul>
      )}
      <MemberPicker key={members?.length ?? 0} groupId={group.id} label={labels.add} placeholder={labels.search} onPick={(id) => change(id, true)} />
      <Message note={note} />
    </li>
  );
}
