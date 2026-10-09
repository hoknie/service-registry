"use client";

import { Pencil, Plus, UserRound } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type Page, type User } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Checkbox, Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { Pager } from "../ui/Pager";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { TokenLabels } from "../account/TokenTable";
import { UserIdentities } from "./UserIdentities";
import { UserTokens } from "./UserTokens";

const PAGE_SIZE = 50;

type Labels = Messages["admin"];

export function StatusBadge({ status, labels }: { status: User["status"]; labels: Labels["status"] }) {
  return (
    <Badge tone={status === "active" ? "signal" : "neutral"} dot>
      {labels[status]}
    </Badge>
  );
}

export function UsersAdmin({ locale, labels, tokenLabels }: { locale: Locale; labels: Labels; tokenLabels: TokenLabels }) {
  const { errors, common } = useUiText();
  const t = labels.users;
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<Page<User> | null>(null);
  const [listError, setListError] = useState<string | null>(null);
  const [editing, setEditing] = useState<User | null>(null);
  const [creating, setCreating] = useState(false);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(
    async (at: number) => {
      try {
        setPage(await apiGet<Page<User>>(`/v1/users?limit=${PAGE_SIZE}&offset=${at}`));
        setListError(null);
      } catch (e) {
        setListError(fail(e));
      }
    },
    [fail],
  );

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(offset);
  }, [load, offset]);

  const newUser = (
    <Button variant="primary" onClick={() => setCreating(true)}>
      <Plus aria-hidden="true" />
      {t.create.title}
    </Button>
  );

  return (
    <>
      <PageHeader glyph={UserRound} title={t.title} lead={t.lead} actions={newUser} />
      {listError && <Message note={{ kind: "error", text: listError }} className="mb-4" />}
      {!page && !listError && <SkeletonTable rows={6} label={common.loading} />}
      {page && page.items.length === 0 && <EmptyState icon={UserRound} title={t.empty} action={newUser} />}
      {page && page.items.length > 0 && (
        <>
          <Table>
            <thead>
              <tr>
                <Th>{t.table.name}</Th>
                <Th>{t.table.status}</Th>
                <Th>{t.table.superadmin}</Th>
                <Th className="text-right">
                  <span className="sr-only">{t.table.actions}</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {page.items.map((u) => (
                <Tr key={u.id} aria-selected={editing?.id === u.id}>
                  <Td>
                    <span className="flex items-center gap-3">
                      <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-full bg-surface-3 text-xs font-semibold text-ink-2">
                        {Array.from(u.display_name.trim())[0]?.toUpperCase() ?? "?"}
                      </span>
                      <span className="min-w-0">
                        <span className="block truncate font-medium text-ink">{u.display_name}</span>
                        <span className="block truncate text-xs text-muted">{u.email}</span>
                      </span>
                      {!u.has_password && (
                        <Badge tone="outline" className="shrink-0">
                          {t.noPassword}
                        </Badge>
                      )}
                    </span>
                  </Td>
                  <Td>
                    <StatusBadge status={u.status} labels={labels.status} />
                  </Td>
                  <Td>{u.is_superadmin ? <Badge tone="amber">{labels.yes}</Badge> : <span className="text-muted">{labels.no}</span>}</Td>
                  <Td className="text-right">
                    <Button size="sm" variant="ghost" onClick={() => setEditing(u)} aria-label={format(t.edit.title, { email: u.email })}>
                      <Pencil aria-hidden="true" />
                      <span className="hidden sm:inline">{t.table.edit}</span>
                    </Button>
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
          <Pager offset={offset} shown={page.items.length} total={page.total} size={PAGE_SIZE} labels={t.pager} onChange={setOffset} />
        </>
      )}

      <CreateUser
        open={creating}
        onOpenChange={setCreating}
        labels={labels}
        fail={fail}
        onCreated={() => {
          setCreating(false);
          toast.success(t.create.done);
          void load(offset);
        }}
      />
      {editing && (
        <EditUser
          key={editing.id}
          user={editing}
          locale={locale}
          labels={labels}
          tokenLabels={tokenLabels}
          fail={fail}
          onSaved={() => void load(offset)}
          onClose={() => setEditing(null)}
        />
      )}
    </>
  );
}

type FormProps = { labels: Labels; fail: (e: unknown) => string };

function CreateUser({
  open,
  onOpenChange,
  labels,
  fail,
  onCreated,
}: FormProps & { open: boolean; onOpenChange: (open: boolean) => void; onCreated: () => void }) {
  const { common } = useUiText();
  const t = labels.users.create;
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [superadmin, setSuperadmin] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      await apiSend("POST", "/v1/users", { email, display_name: name, password, is_superadmin: superadmin });
      setEmail("");
      setName("");
      setPassword("");
      setSuperadmin(false);
      onCreated();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setNote(null);
        onOpenChange(next);
      }}
      title={t.title}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)}>{common.cancel}</Button>
          <Button type="submit" form="create-user" variant="primary" disabled={busy}>
            {busy ? labels.saving : t.submit}
          </Button>
        </>
      }
    >
      <form id="create-user" className="grid gap-4" onSubmit={submit}>
        <Field label={t.email}>
          {(p) => <Input {...p} type="email" required autoComplete="off" value={email} onChange={(e) => setEmail(e.target.value)} />}
        </Field>
        <Field label={t.name}>{(p) => <Input {...p} required value={name} onChange={(e) => setName(e.target.value)} />}</Field>
        <Field label={t.password}>
          {(p) => <Input {...p} type="password" required autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />}
        </Field>
        <Checkbox label={t.superadmin} checked={superadmin} onChange={(e) => setSuperadmin(e.target.checked)} />
        <Message note={note} />
      </form>
    </Dialog>
  );
}

function EditUser({
  user,
  locale,
  labels,
  tokenLabels,
  fail,
  onSaved,
  onClose,
}: FormProps & { user: User; locale: Locale; tokenLabels: TokenLabels; onSaved: () => void; onClose: () => void }) {
  const { common } = useUiText();
  const t = labels.users.edit;
  const r = labels.users.reset;
  const [name, setName] = useState(user.display_name);
  const [status, setStatus] = useState<User["status"]>(user.status);
  const [superadmin, setSuperadmin] = useState(user.is_superadmin);
  const [password, setPassword] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [resetNote, setResetNote] = useState<Note>(null);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    try {
      await apiSend<User>("PATCH", `/v1/users/${user.id}`, { display_name: name, status, is_superadmin: superadmin });
      toast.success(t.done);
      onSaved();
      onClose();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  };

  const reset = async (event: FormEvent) => {
    event.preventDefault();
    try {
      await apiSend("POST", `/v1/users/${user.id}/password`, { password });
      setPassword("");
      setResetNote(null);
      toast.success(r.done);
    } catch (e) {
      setResetNote({ kind: "error", text: fail(e) });
    }
  };

  return (
    <Dialog
      open
      size="lg"
      onOpenChange={(open) => !open && onClose()}
      title={format(t.title, { email: user.email })}
      footer={
        <>
          <Button onClick={onClose}>{common.cancel}</Button>
          <Button type="submit" form="edit-user" variant="primary">
            {t.submit}
          </Button>
        </>
      }
    >
      <form id="edit-user" className="grid gap-4" onSubmit={save}>
        <Field label={t.name}>{(p) => <Input {...p} required value={name} onChange={(e) => setName(e.target.value)} />}</Field>
        <Field label={t.status}>
          {(p) => (
            <Select {...p} value={status} onChange={(e) => setStatus(e.target.value as User["status"])}>
              <option value="active">{labels.status.active}</option>
              <option value="disabled">{labels.status.disabled}</option>
            </Select>
          )}
        </Field>
        <Checkbox label={t.superadmin} checked={superadmin} onChange={(e) => setSuperadmin(e.target.checked)} />
        <Message note={note} />
      </form>
      <form className="mt-6 grid gap-3 border-t border-line pt-5" onSubmit={reset}>
        <h3>{r.title}</h3>
        <div className="flex flex-wrap items-end gap-2">
          <Field label={r.password} className="min-w-0 flex-1">
            {(p) => <Input {...p} type="password" required autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />}
          </Field>
          <Button type="submit">{r.submit}</Button>
        </div>
        <Message note={resetNote} />
      </form>
      <UserIdentities userId={user.id} locale={locale} labels={labels.users.identities} fail={fail} />
      <UserTokens userId={user.id} locale={locale} title={labels.tokens.userTokens} empty={labels.tokens.userTokensEmpty} labels={tokenLabels} fail={fail} />
    </Dialog>
  );
}
