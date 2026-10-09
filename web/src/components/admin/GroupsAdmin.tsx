"use client";

import { Plus, Trash, UserMinus, UsersRound } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type Group, type GroupDetails, type Page, type User } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Combobox } from "../ui/Combobox";
import { Message, type Note } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { StatusBadge } from "./UsersAdmin";

type Labels = Messages["admin"];

export function GroupsAdmin({ labels }: { labels: Labels }) {
  const { errors, common } = useUiText();
  const t = labels.groups;
  const [groups, setGroups] = useState<Page<Group> | null>(null);
  const [listError, setListError] = useState<string | null>(null);
  const [openId, setOpenId] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(async () => {
    try {
      setGroups(await apiGet<Page<Group>>("/v1/groups?limit=200"));
      setListError(null);
    } catch (e) {
      setListError(fail(e));
    }
  }, [fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const newGroup = (
    <Button variant="primary" onClick={() => setCreating(true)}>
      <Plus aria-hidden="true" />
      {t.create.title}
    </Button>
  );

  return (
    <>
      <PageHeader glyph={UsersRound} title={t.title} lead={t.lead} actions={newGroup} />
      {listError && <Message note={{ kind: "error", text: listError }} className="mb-4" />}
      {!groups && !listError && <SkeletonTable rows={4} label={common.loading} />}
      {groups && groups.items.length === 0 && <EmptyState icon={UsersRound} title={t.empty} action={newGroup} />}
      {groups && groups.items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.table.name}</Th>
              <Th numeric className="w-32">{t.table.members}</Th>
              <Th className="w-32">
                <span className="sr-only">{t.table.edit}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {groups.items.map((g) => (
              <Tr key={g.id} aria-selected={openId === g.id}>
                <Td>
                  <span className="flex items-center gap-3 font-medium text-ink">
                    <UsersRound aria-hidden="true" className="size-4 text-muted" />
                    {g.name}
                  </span>
                </Td>
                <Td numeric>{g.member_count}</Td>
                <Td className="text-right">
                  <Button size="sm" variant="ghost" onClick={() => setOpenId(g.id)}>
                    {t.table.edit}
                  </Button>
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}

      <CreateGroup
        open={creating}
        onOpenChange={setCreating}
        labels={labels}
        fail={fail}
        onCreated={(group) => {
          setCreating(false);
          toast.success(t.create.done);
          void load();
          setOpenId(group.id);
        }}
      />
      {openId && (
        <EditGroup
          key={openId}
          id={openId}
          labels={labels}
          fail={fail}
          onChanged={() => void load()}
          onDeleted={() => {
            setOpenId(null);
            toast.success(t.edit.deleted);
            void load();
          }}
          onClose={() => setOpenId(null)}
        />
      )}
    </>
  );
}

type FormProps = { labels: Labels; fail: (e: unknown) => string };

function CreateGroup({
  open,
  onOpenChange,
  labels,
  fail,
  onCreated,
}: FormProps & {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (group: Group) => void;
}) {
  const { common } = useUiText();
  const t = labels.groups.create;
  const [name, setName] = useState("");
  const [note, setNote] = useState<Note>(null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    try {
      const group = await apiSend<Group>("POST", "/v1/groups", { name });
      setName("");
      onCreated(group);
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setNote(null);
        onOpenChange(next);
      }}
      size="sm"
      title={t.title}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)}>{common.cancel}</Button>
          <Button type="submit" form="create-group" variant="primary">
            {t.submit}
          </Button>
        </>
      }
    >
      <form id="create-group" className="grid gap-4" onSubmit={submit}>
        <Field label={t.name}>{(p) => <Input {...p} required value={name} onChange={(e) => setName(e.target.value)} />}</Field>
        <Message note={note} />
      </form>
    </Dialog>
  );
}

type EditProps = FormProps & {
  id: string;
  onChanged: () => void;
  onDeleted: () => void;
  onClose: () => void;
};

function EditGroup({ id, labels, fail, onChanged, onDeleted, onClose }: EditProps) {
  const { common } = useUiText();
  const t = labels.groups;
  const [group, setGroup] = useState<GroupDetails | null>(null);
  const [users, setUsers] = useState<User[]>([]);
  const [name, setName] = useState("");
  const [candidate, setCandidate] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [confirming, setConfirming] = useState(false);

  const load = useCallback(async () => {
    try {
      const details = await apiGet<GroupDetails>(`/v1/groups/${id}`);
      setGroup(details);
      setName(details.name);
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  }, [id, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
    apiGet<Page<User>>("/v1/users?limit=200")
      .then((page) => setUsers(page.items))
      .catch((e: unknown) => setNote({ kind: "error", text: fail(e) }));
  }, [load, fail]);

  const run = async (action: () => Promise<unknown>, success?: string) => {
    try {
      await action();
      setNote(null);
      if (success) toast.success(success);
      await load();
      onChanged();
      return true;
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
      return false;
    }
  };

  const rename = (event: FormEvent) => {
    event.preventDefault();
    void run(() => apiSend("PATCH", `/v1/groups/${id}`, { name }), t.edit.renamed);
  };

  const add = (event: FormEvent) => {
    event.preventDefault();
    if (!candidate) return;
    void run(() => apiSend("PUT", `/v1/groups/${id}/members/${candidate}`)).then((ok) => ok && setCandidate(""));
  };

  const memberIds = new Set(group?.members.map((m) => m.id) ?? []);
  const candidates = users.filter((u) => !memberIds.has(u.id));

  return (
    <Dialog
      open
      onOpenChange={(open) => !open && onClose()}
      size="lg"
      title={group ? format(t.edit.title, { name: group.name }) : common.loading}
      footer={
        group && (
          <>
            <Button variant="danger-ghost" className="mr-auto" onClick={() => setConfirming(true)}>
              <Trash aria-hidden="true" />
              {t.edit.delete}
            </Button>
            <Button onClick={onClose}>{labels.close}</Button>
          </>
        )
      }
    >
      {!group ? (
        note ? (
          <Message note={note} />
        ) : (
          <SkeletonTable rows={4} label={common.loading} />
        )
      ) : (
        <div className="grid gap-6">
          <form className="flex flex-wrap items-end gap-2" onSubmit={rename}>
            <Field label={t.edit.name} className="min-w-0 flex-1">
              {(p) => <Input {...p} required value={name} onChange={(e) => setName(e.target.value)} />}
            </Field>
            <Button type="submit">{t.edit.rename}</Button>
          </form>

          <section className="grid gap-3">
            <h3 className="flex items-center gap-2">
              {t.members.title}
              <Badge>{group.members.length}</Badge>
            </h3>
            {group.members.length === 0 ? (
              <p className="rounded-md border border-dashed border-line-strong px-4 py-5 text-center text-sm text-muted">{t.members.empty}</p>
            ) : (
              <ul className="divide-y divide-line rounded-lg border border-line">
                {group.members.map((m) => (
                  <li key={m.id} className="flex items-center gap-3 px-3 py-2">
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium text-ink">{m.display_name}</span>
                      <span className="block truncate text-xs text-muted">{m.email}</span>
                    </span>
                    {m.status === "disabled" && <StatusBadge status={m.status} labels={labels.status} />}
                    {m.source.startsWith("oauth:") ? (
                      <Badge tone="cobalt" title={t.members.managedHint}>
                        {format(t.members.fromProvider, { provider: m.source.slice("oauth:".length) })}
                      </Badge>
                    ) : (
                      <>
                        <Badge tone="outline">{t.members.manual}</Badge>
                        <Button
                          size="sm"
                          variant="ghost"
                          aria-label={`${t.members.remove}: ${m.display_name}`}
                          onClick={() => void run(() => apiSend("DELETE", `/v1/groups/${id}/members/${m.id}`))}
                        >
                          <UserMinus aria-hidden="true" />
                          <span className="hidden sm:inline">{t.members.remove}</span>
                        </Button>
                      </>
                    )}
                  </li>
                ))}
              </ul>
            )}
            <form className="flex flex-wrap items-end gap-2" onSubmit={add}>
              <Field label={t.members.add} className="min-w-0 flex-1">
                {(p) => (
                  <Combobox
                    {...p}
                    value={candidate}
                    onChange={setCandidate}
                    placeholder={t.members.choose}
                    options={candidates.map((u) => ({ value: u.id, label: u.display_name, detail: u.email, keywords: [u.email] }))}
                  />
                )}
              </Field>
              <Button type="submit" variant="primary" disabled={!candidate}>
                <Plus aria-hidden="true" />
                {t.members.submit}
              </Button>
            </form>
          </section>
          <Message note={note} />
        </div>
      )}
      {group && (
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          title={t.edit.delete}
          text={format(t.edit.confirmDelete, { name: group.name })}
          confirm={t.edit.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/groups/${id}`);
              onDeleted();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </Dialog>
  );
}
