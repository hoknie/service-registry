"use client";

import { Trash, UserMinus, UsersRound } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type GroupDetails } from "@/lib/api";
import { setContactEditor } from "@/lib/ownersApi";
import { useCan, useSuperadmin } from "@/lib/useCan";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { MemberPicker } from "../account/MemberPicker";
import { Checkbox, Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { PageHeader, Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { GroupContacts } from "./GroupContacts";
import { GroupPermissions } from "./GroupPermissions";
import { StatusBadge } from "./UsersAdmin";

type Labels = Messages["admin"];

export function GroupPage({ locale, labels }: { locale: Locale; labels: Labels }) {
  const { errors, common } = useUiText();
  const router = useRouter();
  const id = useSearchParams().get("id") ?? "";
  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const [missing, setMissing] = useState(false);
  const t = labels.groups;
  const [group, setGroup] = useState<GroupDetails | null>(null);
  const [name, setName] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [confirming, setConfirming] = useState(false);
  const can = useCan();
  const superadmin = useSuperadmin();

  const load = useCallback(async () => {
    try {
      const details = await apiGet<GroupDetails>(`/v1/groups/${id}`);
      setGroup(details);
      setName(details.name);
    } catch (e) {
      if (errorCode(e) === "not_found") setMissing(true);
      else setNote({ kind: "error", text: fail(e) });
    }
  }, [id, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const run = async (action: () => Promise<unknown>, success?: string) => {
    try {
      await action();
      setNote(null);
      if (success) toast.success(success);
      await load();
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

  const add = (userId: string) => void run(() => apiSend("PUT", `/v1/groups/${id}/members/${userId}`));

  const back = (
    <Link href={`/${locale}/admin/groups`} className="text-sm text-signal hover:underline">
      {t.page.back}
    </Link>
  );
  if (missing) return <EmptyState icon={UsersRound} title={t.page.notFound} action={back} />;
  if (!group) return note ? <Message note={note} /> : <SkeletonTable rows={4} label={common.loading} />;
  const guarded = !superadmin && Object.keys(group.permissions).length > 0;
  const mayUpdate = can("groups", "update") && !guarded;
  const mayDelete = can("groups", "delete") && !guarded;
  const manage = group.can_manage_members;

  return (
    <>
      <PageHeader
        glyph={UsersRound}
        title={group.name}
        lead={format(t.page.lead, { n: group.members.length })}
        actions={
          mayDelete && (
            <Button variant="danger-ghost" onClick={() => setConfirming(true)}>
              <Trash aria-hidden="true" />
              {t.edit.delete}
            </Button>
          )
        }
      />
      {guarded && <p className="mb-4 rounded-md border border-line bg-surface-2 px-3 py-2 text-sm text-ink-2">{t.permissions.protected}</p>}
      {group.members_lock === "bindings" && !manage && (
        <p className="mb-4 rounded-md border border-line bg-surface-2 px-3 py-2 text-sm text-ink-2">{t.members.lockedBindings}</p>
      )}
      <div className="grid gap-6">
        <Panel title={t.page.settings}>
          <form className="flex flex-wrap items-end gap-2" onSubmit={rename}>
            <Field label={t.edit.name} className="min-w-0 flex-1">
              {(p) => <Input {...p} required value={name} onChange={(e) => setName(e.target.value)} />}
            </Field>
            {mayUpdate && <Button type="submit">{t.edit.rename}</Button>}
          </form>
        </Panel>
        {mayUpdate && <GroupContacts key={group.updated_at} group={group} labels={t.contacts} onSaved={() => void load()} />}
        <Panel title={t.members.title} actions={<Badge>{group.members.length}</Badge>}>
          <section className="grid gap-3">
            {group.members.length === 0 ? (
              <p className="rounded-md border border-dashed border-line-strong px-4 py-5 text-center text-sm text-muted">{t.members.empty}</p>
            ) : (
              <ul className="divide-y divide-line rounded-lg border border-line">
                {group.members.map((m) => {
                  const editorProvider = m.contact_editor_source?.startsWith("oauth:") ? m.contact_editor_source.slice("oauth:".length) : null;
                  return (
                    <li key={m.id} className="flex items-center gap-3 px-3 py-2">
                      <span className="min-w-0 flex-1">
                        <Link href={`/${locale}/admin/users/user?id=${m.id}`} className="block truncate text-sm font-medium text-ink hover:text-signal hover:underline">
                          {m.display_name}
                        </Link>
                        <span className="block truncate text-xs text-muted">{m.email}</span>
                      </span>
                      {m.status === "disabled" && <StatusBadge status={m.status} labels={labels.status} />}
                      <Checkbox
                        label={<span className="hidden sm:inline">{t.members.contactEditor}</span>}
                        aria-label={`${t.members.contactEditor}: ${m.display_name}`}
                        title={editorProvider ? format(t.members.editorFromProvider, { provider: editorProvider }) : t.members.contactEditorHint}
                        checked={m.contact_editor}
                        disabled={editorProvider !== null || !mayUpdate}
                        onChange={(e) => void run(() => setContactEditor(id, m.id, e.target.checked))}
                      />
                      {editorProvider && (
                        <Badge tone="cobalt" title={t.members.managedHint}>
                          {format(t.members.editorFromProvider, { provider: editorProvider })}
                        </Badge>
                      )}
                      {m.source.startsWith("oauth:") ? (
                        <Badge tone="cobalt" title={t.members.managedHint}>
                          {format(t.members.fromProvider, { provider: m.source.slice("oauth:".length) })}
                        </Badge>
                      ) : (
                        <>
                          <Badge tone="outline">{t.members.manual}</Badge>
                          {manage && (
                            <Button
                              size="sm"
                              variant="ghost"
                              aria-label={`${t.members.remove}: ${m.display_name}`}
                              onClick={() => void run(() => apiSend("DELETE", `/v1/groups/${id}/members/${m.id}`))}
                            >
                              <UserMinus aria-hidden="true" />
                              <span className="hidden sm:inline">{t.members.remove}</span>
                            </Button>
                          )}
                        </>
                      )}
                    </li>
                  );
                })}
              </ul>
            )}
            {manage && <MemberPicker key={group.members.length} groupId={group.id} label={t.members.add} placeholder={t.members.search} onPick={add} />}
          </section>
        </Panel>
        <GroupPermissions
          key={JSON.stringify(group.permissions)}
          groupId={group.id}
          initial={group.permissions}
          editable={superadmin}
          labels={t.permissions}
          onSaved={() => void load()}
        />
        <Message note={note} />
      </div>
      {group && mayDelete && (
        <ConfirmDialog
          open={confirming}
          onOpenChange={setConfirming}
          title={t.edit.delete}
          text={format(t.edit.confirmDelete, { name: group.name })}
          confirm={t.edit.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/groups/${id}`);
              toast.success(t.edit.deleted);
              router.push(`/${locale}/admin/groups`);
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </>
  );
}
