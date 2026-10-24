"use client";

import { ArrowLeft, Plus, UsersRound } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { format } from "@/i18n/format";
import type { CatalogNode, GroupDetails, OrganizationGroups as GroupList } from "@/lib/api";
import {
  createOrganizationGroup,
  deleteOrganizationGroup,
  organizationCandidates,
  organizationGroup,
  organizationGroups,
  renameOrganizationGroup,
  setOrganizationMember,
} from "@/lib/organizationGroups";

import { MemberPicker } from "../account/MemberPicker";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { CatalogLabels } from "./shared";

type Props = { node: CatalogNode; labels: CatalogLabels; fail: (e: unknown) => string };

export function OrganizationGroups({ node, labels, fail }: Props) {
  const { common } = useUiText();
  const t = labels.orgGroups;
  const [list, setList] = useState<GroupList | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [open, setOpen] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setList(await organizationGroups(node.id));
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [node.id, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const create = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    try {
      await createOrganizationGroup(node.id, name.trim());
      setName("");
      toast.success(t.created);
      await load();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  };

  if (open) {
    return (
      <GroupMembers
        node={node}
        groupId={open}
        labels={labels}
        fail={fail}
        canManage={!!list?.can_manage}
        onBack={() => {
          setOpen(null);
          void load();
        }}
      />
    );
  }

  return (
    <Panel title={t.title} description={t.lead} bodyClassName="grid gap-4">
      {error && <Message note={{ kind: "error", text: error }} />}
      {!list && !error && <SkeletonTable rows={3} label={common.loading} />}
      {list && list.can_manage && (
        <form className="flex flex-wrap items-end gap-2" onSubmit={create}>
          <Input
            required
            maxLength={100}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t.newName}
            aria-label={t.newName}
            className="min-w-0 flex-1"
          />
          <Button type="submit" variant="primary">
            <Plus aria-hidden="true" />
            {t.create}
          </Button>
          <Message note={note} className="basis-full" />
        </form>
      )}
      {list && !list.can_manage && <p className="text-sm text-muted">{t.readOnly}</p>}
      {list && list.items.length === 0 && <EmptyState icon={UsersRound} title={t.empty} />}
      {list && list.items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.name}</Th>
              <Th numeric>{t.members}</Th>
              <Th>
                <span className="sr-only">{t.open}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {list.items.map((g) => (
              <Tr key={g.id}>
                <Td className="font-medium text-ink">{g.name}</Td>
                <Td numeric>{g.member_count}</Td>
                <Td className="text-right">
                  <Button size="sm" variant="ghost" onClick={() => setOpen(g.id)}>
                    {t.open}
                  </Button>
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
    </Panel>
  );
}

function GroupMembers({
  node,
  groupId,
  labels,
  fail,
  canManage,
  onBack,
}: {
  node: CatalogNode;
  groupId: string;
  labels: CatalogLabels;
  fail: (e: unknown) => string;
  canManage: boolean;
  onBack: () => void;
}) {
  const { common } = useUiText();
  const t = labels.orgGroups;
  const [group, setGroup] = useState<GroupDetails | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [deleting, setDeleting] = useState(false);

  const load = useCallback(async () => {
    try {
      const g = await organizationGroup(node.id, groupId);
      setGroup(g);
      setName(g.name);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [node.id, groupId, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const candidates = useCallback((q: string, signal: AbortSignal) => organizationCandidates(node.id, groupId, q, signal), [node.id, groupId]);

  const member = async (userId: string, on: boolean) => {
    try {
      await setOrganizationMember(node.id, groupId, userId, on);
      toast.success(on ? t.added : t.removed);
      await load();
    } catch (e) {
      toast.error(fail(e));
    }
  };

  const rename = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    try {
      await renameOrganizationGroup(node.id, groupId, name.trim());
      toast.success(t.renamed);
      await load();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  };

  const back = (
    <Button size="sm" variant="ghost" onClick={onBack}>
      <ArrowLeft aria-hidden="true" />
      {t.back}
    </Button>
  );

  return (
    <Panel title={group ? format(t.membersTitle, { name: group.name }) : t.title} actions={back} bodyClassName="grid gap-4">
      {error && <Message note={{ kind: "error", text: error }} />}
      {!group && !error && <SkeletonTable rows={3} label={common.loading} />}
      {group && canManage && (
        <form className="flex flex-wrap items-end gap-2" onSubmit={rename}>
          <Input required maxLength={100} value={name} onChange={(e) => setName(e.target.value)} aria-label={t.name} className="min-w-0 flex-1" />
          <Button type="submit">{t.rename}</Button>
          <Button variant="danger-ghost" onClick={() => setDeleting(true)}>
            {t.delete}
          </Button>
          <Message note={note} className="basis-full" />
        </form>
      )}
      {group && group.can_manage_members && (
        <MemberPicker groupId={groupId} label={t.addMember} placeholder={t.search} onPick={(id) => void member(id, true)} candidates={candidates} minChars={3} />
      )}
      {group && group.members_hidden && <p className="text-sm text-muted">{t.membersHidden}</p>}
      {group && !group.members_hidden && group.members.length === 0 && <EmptyState icon={UsersRound} title={t.membersEmpty} />}
      {group && group.members.length > 0 && (
        <Table>
          <tbody>
            {group.members.map((m) => (
              <Tr key={m.id}>
                <Td>
                  <span className="font-medium text-ink">{m.display_name}</span>
                  <span className="ml-2 text-muted">{m.email}</span>
                </Td>
                <Td className="text-right">
                  {group.can_manage_members && (
                    <Button size="sm" variant="danger-ghost" onClick={() => void member(m.id, false)}>
                      {t.remove}
                    </Button>
                  )}
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
      {group && (
        <ConfirmDialog
          open={deleting}
          onOpenChange={setDeleting}
          title={t.delete}
          text={format(t.confirmDelete, { name: group.name })}
          confirm={t.delete}
          onConfirm={async () => {
            try {
              await deleteOrganizationGroup(node.id, groupId);
              toast.success(t.deleted);
              onBack();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </Panel>
  );
}
