"use client";

import { UserRound, UsersRound } from "lucide-react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type Group, type Items, type User } from "@/lib/api";

import type { TokenLabels } from "../account/TokenTable";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Checkbox, Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { PageHeader, Panel } from "../ui/Panel";
import { Select } from "../ui/Select";
import { SkeletonTable } from "../ui/Skeleton";
import { StatusBadge } from "./UsersAdmin";
import { UserIdentities } from "./UserIdentities";
import { UserTokens } from "./UserTokens";

type Labels = Messages["admin"];

export function UserPage({ locale, labels, tokenLabels }: { locale: Locale; labels: Labels; tokenLabels: TokenLabels }) {
  const { errors, common } = useUiText();
  const id = useSearchParams().get("id") ?? "";
  const t = labels.users.edit;
  const r = labels.users.reset;
  const p = labels.users.page;
  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const [user, setUser] = useState<User | null>(null);
  const [missing, setMissing] = useState(false);
  const [groups, setGroups] = useState<Group[] | null>(null);
  const [name, setName] = useState("");
  const [status, setStatus] = useState<User["status"]>("active");
  const [superadmin, setSuperadmin] = useState(false);
  const [service, setService] = useState(false);
  const [password, setPassword] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [resetNote, setResetNote] = useState<Note>(null);

  const show = (u: User) => {
    setUser(u);
    setName(u.display_name);
    setStatus(u.status);
    setSuperadmin(u.is_superadmin);
    setService(u.is_service);
  };

  const load = useCallback(async () => {
    if (!id) {
      setMissing(true);
      return;
    }
    try {
      show(await apiGet<User>(`/v1/users/${id}`));
      setGroups((await apiGet<Items<Group>>(`/v1/users/${id}/groups`)).items);
    } catch (e) {
      if (errorCode(e) === "not_found") setMissing(true);
      else setNote({ kind: "error", text: fail(e) });
    }
  }, [id, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    try {
      show(await apiSend<User>("PATCH", `/v1/users/${id}`, { display_name: name, status, is_superadmin: superadmin, is_service: service }));
      toast.success(t.done);
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  };

  const reset = async (event: FormEvent) => {
    event.preventDefault();
    try {
      await apiSend("POST", `/v1/users/${id}/password`, { password });
      setPassword("");
      setResetNote(null);
      toast.success(r.done);
      void load();
    } catch (e) {
      setResetNote({ kind: "error", text: fail(e) });
    }
  };

  const back = (
    <Link href={`/${locale}/admin/users`} className="text-sm text-signal hover:underline">
      {p.back}
    </Link>
  );
  if (missing) return <EmptyState icon={UserRound} title={p.notFound} action={back} />;
  if (!user) return note ? <Message note={note} /> : <SkeletonTable rows={4} label={common.loading} />;

  return (
    <>
      <PageHeader glyph={UserRound} title={user.display_name} lead={user.email} />
      <div className="mb-6 flex flex-wrap gap-2">
        <StatusBadge status={user.status} labels={labels.status} />
        {user.is_superadmin && <Badge tone="amber">{p.superadmin}</Badge>}
        {user.is_service && <Badge tone="cobalt">{p.service}</Badge>}
        {!user.has_password && <Badge tone="outline">{labels.users.noPassword}</Badge>}
      </div>
      <div className="grid items-start gap-6 lg:grid-cols-2">
        <Panel title={p.profile}>
          <form className="grid gap-4" onSubmit={save}>
            <Field label={t.name}>{(f) => <Input {...f} required value={name} onChange={(e) => setName(e.target.value)} />}</Field>
            <Field label={t.status}>
              {(f) => (
                <Select {...f} value={status} onChange={(e) => setStatus(e.target.value as User["status"])}>
                  <option value="active">{labels.status.active}</option>
                  <option value="disabled">{labels.status.disabled}</option>
                </Select>
              )}
            </Field>
            <Checkbox label={t.superadmin} checked={superadmin} onChange={(e) => setSuperadmin(e.target.checked)} />
            <div className="grid gap-1">
              <Checkbox label={p.serviceToggle} checked={service} onChange={(e) => setService(e.target.checked)} />
              <p className="pl-6.5 text-xs text-muted">{p.serviceHint}</p>
            </div>
            <Message note={note} />
            <div>
              <Button type="submit" variant="primary">
                {t.submit}
              </Button>
            </div>
          </form>
        </Panel>
        <div className="grid gap-6">
          {!user.is_service && (
            <Panel title={r.title}>
              <form className="grid gap-3" onSubmit={reset}>
                <div className="flex flex-wrap items-end gap-2">
                  <Field label={r.password} className="min-w-0 flex-1">
                    {(f) => <Input {...f} type="password" required autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />}
                  </Field>
                  <Button type="submit">{r.submit}</Button>
                </div>
                <Message note={resetNote} />
              </form>
            </Panel>
          )}
          <Panel title={p.groups}>
            {groups && groups.length === 0 && <p className="text-sm text-muted">{p.noGroups}</p>}
            {groups && groups.length > 0 && (
              <ul className="flex flex-wrap gap-2">
                {groups.map((g) => (
                  <li key={g.id}>
                    <Link
                      href={`/${locale}/admin/groups/group?id=${g.id}`}
                      className="inline-flex items-center gap-1.5 rounded-md border border-line px-2.5 py-1 text-sm text-ink hover:border-signal hover:text-signal"
                    >
                      <UsersRound aria-hidden="true" className="size-3.5" />
                      {g.name}
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
        </div>
      </div>
      <div className="mt-6 grid gap-6">
        <Panel title={labels.users.identities.title}>
          <UserIdentities userId={user.id} locale={locale} labels={labels.users.identities} fail={fail} />
        </Panel>
        <div className="rounded-card border border-line bg-surface p-5 shadow-elev-1">
          <UserTokens
            userId={user.id}
            locale={locale}
            title={labels.tokens.userTokens}
            empty={labels.tokens.userTokensEmpty}
            labels={tokenLabels}
            fail={fail}
            canIssue={user.status === "active"}
            superadmin={user.is_superadmin}
          />
        </div>
      </div>
    </>
  );
}
