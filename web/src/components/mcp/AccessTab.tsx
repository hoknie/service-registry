"use client";

import { User, UsersRound, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiGet, errorCode, type Items } from "@/lib/api";
import type { Suggestion } from "@/lib/labelSuggest";
import type { McpAccess, McpServer } from "@/lib/mcp";
import { loadAccess, saveAccess } from "@/lib/mcpApi";
import { ownerCandidates } from "@/lib/ownersApi";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Checkbox, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonRows } from "../ui/Skeleton";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";

type Subject = { kind: "group" | "user"; id: string; label: string };

async function globalCandidates(
  q: string,
  signal: AbortSignal,
): Promise<Subject[]> {
  const needle = q.trim().toLowerCase();
  const [groups, users] = await Promise.all([
    apiGet<Items<{ id: string; name: string }>>(
      "/v1/groups?limit=200",
      signal,
    ).catch(() => ({ items: [] })),
    apiGet<Items<{ id: string; email: string }>>(
      "/v1/users?limit=200",
      signal,
    ).catch(() => ({ items: [] })),
  ]);
  return [
    ...groups.items.map((g): Subject => ({
      kind: "group",
      id: g.id,
      label: g.name,
    })),
    ...users.items.map((u): Subject => ({
      kind: "user",
      id: u.id,
      label: u.email,
    })),
  ].filter((s) => s.label.toLowerCase().includes(needle));
}

function SubjectPicker({
  organization,
  label,
  exclude,
  onPick,
}: {
  organization: string | null;
  label: string;
  exclude: Subject[];
  onPick: (s: Subject) => void;
}) {
  const [draft, setDraft] = useState("");
  const [focused, setFocused] = useState(false);
  const suggest = useCallback(
    async (q: string, signal: AbortSignal): Promise<Suggestion[]> => {
      const found = organization
        ? (await ownerCandidates(organization, q, signal)).map(
            (c): Subject => ({
              kind: c.subject_kind === "group" ? "group" : "user",
              id: c.subject_id,
              label: c.subject_kind === "group" ? c.name : (c.email ?? c.name),
            }),
          )
        : await globalCandidates(q, signal);
      return found
        .filter((s) => !exclude.some((e) => e.kind === s.kind && e.id === s.id))
        .slice(0, 20)
        .map((s) => ({ value: `${s.kind}:${s.id}`, label: s.label }));
    },
    [organization, exclude],
  );
  const hints = useSuggestions(draft, suggest, focused);
  const choose = (s: Suggestion) => {
    const [kind, id = ""] = s.value.split(":");
    onPick({ kind: kind === "group" ? "group" : "user", id, label: s.label });
    setDraft("");
    hints.close();
  };
  return (
    <div className="relative">
      <Input
        {...hints.inputProps}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={(e) => hints.onKey(e, choose)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        placeholder={label}
        aria-label={label}
        autoComplete="off"
        className="w-full"
      />
      {hints.open && (
        <SuggestionList
          id={hints.listId}
          items={hints.items}
          active={hints.active}
          onActive={hints.setActive}
          onChoose={choose}
          icon={(s) =>
            s.value.startsWith("group:") ? (
              <UsersRound className="size-4" />
            ) : (
              <User className="size-4" />
            )
          }
        />
      )}
    </div>
  );
}

function subjectsOf(a: McpAccess): Subject[] {
  return [
    ...a.groups.map((g): Subject => ({
      kind: "group",
      id: g.id,
      label: g.name,
    })),
    ...a.users.map((u): Subject => ({
      kind: "user",
      id: u.id,
      label: u.email,
    })),
  ];
}

export function AccessTab({
  server,
  labels: t,
}: {
  server: McpServer;
  labels: Messages["mcp"];
}) {
  const x = t.access;
  const { errors } = useUiText();
  const [access, setAccess] = useState<McpAccess | null>(null);
  const [everyone, setEveryone] = useState(true);
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  useEffect(() => {
    loadAccess(server.id)
      .then((a) => {
        setAccess(a);
        setEveryone(a.everyone);
        setSubjects(subjectsOf(a));
      })
      .catch((e: unknown) =>
        setNote({ kind: "error", text: errorText(errors, errorCode(e)) }),
      );
  }, [server.id, errors]);

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      const a = await saveAccess(server.id, {
        everyone,
        groups: subjects.filter((s) => s.kind === "group").map((s) => s.id),
        users: subjects.filter((s) => s.kind === "user").map((s) => s.id),
      });
      setAccess(a);
      setSubjects(subjectsOf(a));
      setNote({ kind: "success", text: x.saved });
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  if (!access)
    return note ? (
      <Message note={note} />
    ) : (
      <SkeletonRows rows={3} label={x.title} />
    );
  const manage = access.can_manage;
  return (
    <Panel title={x.title} description={manage ? x.lead : x.readOnly}>
      <div className="grid gap-4">
        <Checkbox
          label={x.everyone}
          checked={everyone}
          disabled={!manage}
          onChange={(e) => setEveryone(e.target.checked)}
        />
        <p className="text-sm text-muted">{x.everyoneHint}</p>
        {!everyone && (
          <>
            <ul className="flex flex-wrap gap-2">
              {subjects.map((s) => (
                <li
                  key={`${s.kind}:${s.id}`}
                  className="inline-flex items-center gap-1.5 rounded-full border border-line bg-surface-2 px-3 py-1 text-sm"
                >
                  {s.kind === "group" ? (
                    <UsersRound aria-hidden="true" className="size-3.5" />
                  ) : (
                    <User aria-hidden="true" className="size-3.5" />
                  )}
                  {s.label}
                  {manage && (
                    <button
                      type="button"
                      aria-label={`${x.remove} ${s.label}`}
                      className="text-muted hover:text-danger"
                      onClick={() =>
                        setSubjects(subjects.filter((o) => o !== s))
                      }
                    >
                      <X aria-hidden="true" className="size-3.5" />
                    </button>
                  )}
                </li>
              ))}
              {subjects.length === 0 && (
                <li className="text-sm text-muted">{x.nobody}</li>
              )}
            </ul>
            {manage && (
              <SubjectPicker
                organization={server.organization_id}
                label={x.add}
                exclude={subjects}
                onPick={(s) => setSubjects([...subjects, s])}
              />
            )}
          </>
        )}
        <Message note={note} />
        {manage && (
          <div>
            <Button variant="primary" busy={busy} onClick={save}>
              {x.save}
            </Button>
          </div>
        )}
      </div>
    </Panel>
  );
}
