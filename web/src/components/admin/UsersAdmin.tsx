"use client";

import { Plus, UserRound } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
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
import { Message, type Note } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { Pager } from "../ui/Pager";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";

const PAGE_SIZE = 50;

type Labels = Messages["admin"];

export function StatusBadge({ status, labels }: { status: User["status"]; labels: Labels["status"] }) {
  return (
    <Badge tone={status === "active" ? "signal" : "neutral"} dot>
      {labels[status]}
    </Badge>
  );
}

export function UsersAdmin({ locale, labels }: { locale: Locale; labels: Labels }) {
  const { errors, common } = useUiText();
  const router = useRouter();
  const t = labels.users;
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<Page<User> | null>(null);
  const [listError, setListError] = useState<string | null>(null);
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
                <Tr key={u.id}>
                  <Td>
                    <Link href={`/${locale}/admin/users/user?id=${u.id}`} className="group flex items-center gap-3">
                      <span aria-hidden="true" className="grid size-8 shrink-0 place-items-center rounded-full bg-surface-3 text-xs font-semibold text-ink-2">
                        {Array.from(u.display_name.trim())[0]?.toUpperCase() ?? "?"}
                      </span>
                      <span className="min-w-0">
                        <span className="block truncate font-medium text-ink group-hover:text-signal group-hover:underline">{u.display_name}</span>
                        <span className="block truncate text-xs text-muted">{u.email}</span>
                      </span>
                      {u.is_service && (
                        <Badge tone="cobalt" className="shrink-0">
                          {t.page.service}
                        </Badge>
                      )}
                      {!u.has_password && !u.is_service && (
                        <Badge tone="outline" className="shrink-0">
                          {t.noPassword}
                        </Badge>
                      )}
                    </Link>
                  </Td>
                  <Td>
                    <StatusBadge status={u.status} labels={labels.status} />
                  </Td>
                  <Td>{u.is_superadmin ? <Badge tone="amber">{labels.yes}</Badge> : <span className="text-muted">{labels.no}</span>}</Td>
                  <Td className="text-right">
                    <Link
                      href={`/${locale}/admin/users/user?id=${u.id}`}
                      aria-label={format(t.edit.title, { email: u.email })}
                      className="text-sm text-signal hover:underline"
                    >
                      {t.table.edit}
                    </Link>
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
        onCreated={(user) => {
          setCreating(false);
          toast.success(t.create.done);
          router.push(`/${locale}/admin/users/user?id=${user.id}`);
        }}
      />
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
}: FormProps & { open: boolean; onOpenChange: (open: boolean) => void; onCreated: (user: User) => void }) {
  const { common } = useUiText();
  const t = labels.users.create;
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [superadmin, setSuperadmin] = useState(false);
  const [service, setService] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const user = await apiSend<User>("POST", "/v1/users", {
        email,
        display_name: name,
        ...(password || !service ? { password } : {}),
        is_superadmin: superadmin,
        is_service: service,
      });
      setEmail("");
      setName("");
      setPassword("");
      setSuperadmin(false);
      setService(false);
      onCreated(user);
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
          {(p) => (
            <Input {...p} type="password" required={!service} autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />
          )}
        </Field>
        <Checkbox label={t.superadmin} checked={superadmin} onChange={(e) => setSuperadmin(e.target.checked)} />
        <div className="grid gap-1">
          <Checkbox label={labels.users.page.serviceToggle} checked={service} onChange={(e) => setService(e.target.checked)} />
          <p className="pl-6.5 text-xs text-muted">{labels.users.page.serviceCreateHint}</p>
        </div>
        <Message note={note} />
      </form>
    </Dialog>
  );
}
