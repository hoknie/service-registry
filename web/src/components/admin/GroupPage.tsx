"use client";

import { Plus, Trash, UserMinus, UsersRound } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type GroupDetails, type Page, type User } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Combobox } from "../ui/Combobox";
import { Message, type Note } from "../ui/Message";
import { PageHeader, Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
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
      if (errorCode(e) === "not_found") setMissing(true);
      else setNote({ kind: "error", text: fail(e) });
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

  const back = (
    <Link href={`/${locale}/admin/groups`} className="text-sm text-signal hover:underline">
      {t.page.back}
    </Link>
  );
  if (missing) return <EmptyState icon={UsersRound} title={t.page.notFound} action={back} />;
  if (!group) return note ? <Message note={note} /> : <SkeletonTable rows={4} label={common.loading} />;

  return (
    <>
      <PageHeader
        glyph={UsersRound}
        title={group.name}
        lead={format(t.page.lead, { n: group.members.length })}
        actions={
          <Button variant="danger-ghost" onClick={() => setConfirming(true)}>
            <Trash aria-hidden="true" />
            {t.edit.delete}
          </Button>
        }
      />
      <div className="grid gap-6">
        <Panel title={t.page.settings}>
          <form className="flex flex-wrap items-end gap-2" onSubmit={rename}>
            <Field label={t.edit.name} className="min-w-0 flex-1">
              {(p) => <Input {...p} required value={name} onChange={(e) => setName(e.target.value)} />}
            </Field>
            <Button type="submit">{t.edit.rename}</Button>
          </form>
        </Panel>
        <Panel title={t.members.title} actions={<Badge>{group.members.length}</Badge>}>
          <section className="grid gap-3">
            {group.members.length === 0 ? (
              <p className="rounded-md border border-dashed border-line-strong px-4 py-5 text-center text-sm text-muted">{t.members.empty}</p>
            ) : (
              <ul className="divide-y divide-line rounded-lg border border-line">
                {group.members.map((m) => (
                  <li key={m.id} className="flex items-center gap-3 px-3 py-2">
                    <span className="min-w-0 flex-1">
                      <Link href={`/${locale}/admin/users/user?id=${m.id}`} className="block truncate text-sm font-medium text-ink hover:text-signal hover:underline">
                        {m.display_name}
                      </Link>
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
        </Panel>
        <Message note={note} />
      </div>
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
