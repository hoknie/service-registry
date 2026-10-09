"use client";

import { Plus, UsersRound } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type Group, type Page } from "@/lib/api";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";

type Labels = Messages["admin"];

export function GroupsAdmin({ locale, labels }: { locale: Locale; labels: Labels }) {
  const { errors, common } = useUiText();
  const router = useRouter();
  const t = labels.groups;
  const [groups, setGroups] = useState<Page<Group> | null>(null);
  const [listError, setListError] = useState<string | null>(null);
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
              <Tr key={g.id}>
                <Td>
                  <Link href={`/${locale}/admin/groups/group?id=${g.id}`} className="flex items-center gap-3 font-medium text-ink hover:text-signal hover:underline">
                    <UsersRound aria-hidden="true" className="size-4 text-muted" />
                    {g.name}
                  </Link>
                </Td>
                <Td numeric>{g.member_count}</Td>
                <Td className="text-right">
                  <Link href={`/${locale}/admin/groups/group?id=${g.id}`} className="text-sm text-signal hover:underline">
                    {t.table.edit}
                  </Link>
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
          router.push(`/${locale}/admin/groups/group?id=${group.id}`);
        }}
      />
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
