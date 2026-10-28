"use client";

import { MailPlus, Trash2, UserPlus, UsersRound } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { errorCode, type CatalogNode } from "@/lib/api";
import { displayName, MEMBER_ROLES, type GroupAccess, type Invitation, type MemberPage, type MemberRole, type OrgMember } from "@/lib/members";
import { loadGroupsWithAccess, loadInvitations, loadMembers, removeMember, setMemberRole } from "@/lib/membersApi";

import { AddMemberDialog } from "../members/AddMemberDialog";
import { InvitationsBlock } from "../members/InvitationsBlock";
import { InviteDialog } from "../members/InviteDialog";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { CatalogLabels } from "./shared";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels };

const NO_ROLE = "none";

export function OrganizationMembers({ node, locale, labels }: Props) {
  const t = labels.members;
  const roles = labels.roles as Record<MemberRole, string>;
  const { errors, common } = useUiText();
  const [query, setQuery] = useState("");
  const [applied, setApplied] = useState("");
  const [page, setPage] = useState<MemberPage | null>(null);
  const [groups, setGroups] = useState<GroupAccess[]>([]);
  const [invitations, setInvitations] = useState<Invitation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [inviting, setInviting] = useState<string | null>(null);
  const [removing, setRemoving] = useState<OrgMember | null>(null);
  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  useEffect(() => {
    const timer = setTimeout(() => setApplied(query), 300);
    return () => clearTimeout(timer);
  }, [query]);

  const load = useCallback(async () => {
    try {
      const p = await loadMembers(node.id, applied, null);
      setPage(p);
      setError(null);
      setGroups((await loadGroupsWithAccess(node.id)).items);
      setInvitations(p.can_manage ? (await loadInvitations(node.id)).items : []);
    } catch (e) {
      setError(fail(e));
    }
  }, [node.id, applied, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const more = async () => {
    if (!page?.next_cursor) return;
    try {
      const next = await loadMembers(node.id, applied, page.next_cursor);
      setPage({ ...next, items: [...page.items, ...next.items] });
    } catch (e) {
      toast.error(fail(e));
    }
  };

  const changeRole = async (m: OrgMember, value: string) => {
    try {
      await setMemberRole(node.id, m.user.id, value === NO_ROLE ? null : (value as MemberRole));
      toast.success(t.roleChanged);
      await load();
    } catch (e) {
      toast.error(fail(e));
    }
  };

  const remove = async (): Promise<string | null> => {
    if (!removing) return null;
    try {
      await removeMember(node.id, removing.user.id);
      toast.success(t.removed);
      await load();
      return null;
    } catch (e) {
      return fail(e);
    }
  };

  const manage = page?.can_manage ?? false;
  return (
    <div className="grid gap-6">
      <Panel
        title={t.title}
        description={t.lead}
        actions={
          manage && (
            <>
              <Button onClick={() => setInviting("")}>
                <MailPlus aria-hidden="true" />
                {t.invite}
              </Button>
              <Button variant="primary" onClick={() => setAdding(true)}>
                <UserPlus aria-hidden="true" />
                {t.add}
              </Button>
            </>
          )
        }
        bodyClassName="p-0 sm:p-0"
      >
        <div className="border-b border-line p-4">
          <Input type="search" value={query} onChange={(e) => setQuery(e.target.value)} placeholder={t.search} aria-label={t.search} className="w-72 max-w-full" />
        </div>
        {error && <Message note={{ kind: "error", text: error }} className="m-4" />}
        {!page && !error && <SkeletonTable rows={4} label={common.loading} className="p-5" />}
        {page && page.items.length === 0 && <EmptyState icon={UsersRound} title={t.empty} className="m-5" />}
        {page && page.items.length > 0 && (
          <div className="[&>div]:rounded-none [&>div]:border-0">
            <Table>
              <thead>
                <Tr>
                  <Th>{t.name}</Th>
                  <Th className="hidden md:table-cell">{t.email}</Th>
                  <Th>{t.role}</Th>
                  <Th className="hidden sm:table-cell">{t.source}</Th>
                  <Th className="hidden lg:table-cell">{t.since}</Th>
                  {manage && <Th />}
                </Tr>
              </thead>
              <tbody>
                {page.items.map((m) => (
                  <Tr key={m.user.id}>
                    <Td>
                      <span className="font-medium text-ink" title={m.user.email}>
                        {displayName(m.user)}
                      </span>
                      {m.user.service && (
                        <Badge tone="outline" className="ml-2">
                          {t.service}
                        </Badge>
                      )}
                    </Td>
                    <Td className="hidden text-muted md:table-cell">{m.user.email}</Td>
                    <Td>
                      {manage ? (
                        <Select aria-label={t.role} className="w-40" value={m.role ?? NO_ROLE} onChange={(e) => void changeRole(m, e.target.value)}>
                          <option value={NO_ROLE}>{t.noRole}</option>
                          {MEMBER_ROLES.map((r) => (
                            <option key={r} value={r}>
                              {roles[r]}
                            </option>
                          ))}
                        </Select>
                      ) : m.role ? (
                        roles[m.role]
                      ) : (
                        <span className="text-muted">{t.noRole}</span>
                      )}
                    </Td>
                    <Td className="hidden text-muted sm:table-cell">{t.sources[m.source]}</Td>
                    <Td className="hidden text-muted lg:table-cell">
                      <span title={when(m.added_at, locale, "")}>{ago(m.added_at, locale, "")}</span>
                    </Td>
                    {manage && (
                      <Td>
                        <Button size="sm" variant="ghost" aria-label={format(t.removeOf, { name: displayName(m.user) })} onClick={() => setRemoving(m)}>
                          <Trash2 aria-hidden="true" />
                        </Button>
                      </Td>
                    )}
                  </Tr>
                ))}
              </tbody>
            </Table>
          </div>
        )}
        {page?.next_cursor && (
          <div className="flex justify-center p-4">
            <Button onClick={more}>{t.more}</Button>
          </div>
        )}
      </Panel>
      {manage && <InvitationsBlock org={node.id} items={invitations} locale={locale} roles={roles} labels={t} onChanged={() => void load()} />}
      {groups.length > 0 && (
        <Panel title={t.viaGroups} description={t.viaGroupsLead}>
          <ul className="grid gap-2 text-sm">
            {groups.map((g) => (
              <li key={g.id} className="flex flex-wrap items-center justify-between gap-2">
                <span className="font-medium text-ink">{g.name}</span>
                <span className="text-muted">
                  {roles[g.role]} · {format(t.users, { count: g.users })}
                </span>
              </li>
            ))}
          </ul>
        </Panel>
      )}
      <AddMemberDialog
        open={adding}
        onOpenChange={setAdding}
        org={node.id}
        roles={roles}
        labels={t}
        onAdded={() => {
          toast.success(t.added);
          void load();
        }}
        onInvite={(email) => {
          setAdding(false);
          setInviting(email);
        }}
      />
      <InviteDialog
        open={inviting !== null}
        onOpenChange={(o) => !o && setInviting(null)}
        org={node.id}
        locale={locale}
        email={inviting ?? ""}
        roles={roles}
        labels={t}
        onInvited={() => void load()}
      />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(o) => !o && setRemoving(null)}
        title={format(t.removeTitle, { name: removing ? displayName(removing.user) : "" })}
        text={t.removeText}
        confirm={t.remove}
        onConfirm={remove}
      />
    </div>
  );
}
