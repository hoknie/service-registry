"use client";

import { Plus, ShieldCheck, UserRound, UsersRound } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type Binding, type Group, type Items, type Page, type Role, type User } from "@/lib/api";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Combobox } from "../ui/Combobox";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { CatalogLabels } from "./shared";

const ROLES: Role[] = ["viewer", "editor", "admin"];
const roleTone: Record<Role, Tone> = { viewer: "neutral", editor: "cobalt", admin: "amber" };

type Props = { nodeId: string; labels: CatalogLabels };

export function AccessTab({ nodeId, labels }: Props) {
  const { errors, common } = useUiText();
  const t = labels.access;
  const [bindings, setBindings] = useState<Binding[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [granting, setGranting] = useState(false);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(async () => {
    try {
      setBindings((await apiGet<Items<Binding>>(`/v1/catalog/nodes/${nodeId}/bindings`)).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [nodeId, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const remove = async (b: Binding) => {
    try {
      await apiSend("DELETE", `/v1/catalog/nodes/${nodeId}/bindings/${b.id}`);
      toast.success(t.removed);
      await load();
    } catch (e) {
      toast.error(fail(e));
    }
  };

  const grantButton = (
    <Button variant="primary" onClick={() => setGranting(true)}>
      <Plus aria-hidden="true" />
      {t.add}
    </Button>
  );

  return (
    <Panel title={t.title} description={t.lead} actions={grantButton} bodyClassName="p-0 sm:p-0">
      {error && <Message note={{ kind: "error", text: error }} className="m-4" />}
      {!bindings && !error && <SkeletonTable rows={3} label={common.loading} className="p-5" />}
      {bindings && bindings.length === 0 && <EmptyState icon={ShieldCheck} title={t.empty} className="m-5" />}
      {bindings && bindings.length > 0 && (
        <div className="[&>div]:rounded-none [&>div]:border-0">
          <Table>
            <thead>
              <tr>
                <Th>{t.subject}</Th>
                <Th>{t.role}</Th>
                <Th>{t.source}</Th>
                <Th>
                  <span className="sr-only">{t.remove}</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {bindings.map((b) => {
                const Icon = b.subject_kind === "user" ? UserRound : UsersRound;
                return (
                  <Tr key={b.id}>
                    <Td>
                      <span className="flex items-center gap-2.5">
                        <Icon aria-hidden="true" className="size-4 shrink-0 text-muted" />
                        <span className="font-medium text-ink">{b.subject_name}</span>
                        <span className="sr-only">({b.subject_kind === "user" ? t.user : t.group})</span>
                      </span>
                    </Td>
                    <Td>
                      <Badge tone={roleTone[b.role]}>{labels.roles[b.role]}</Badge>
                    </Td>
                    <Td className={b.inherited ? "text-muted" : "text-ink-2"}>{b.inherited ? format(t.inherited, { name: b.node_name }) : t.here}</Td>
                    <Td className="text-right">
                      {!b.inherited && (
                        <Button size="sm" variant="danger-ghost" onClick={() => void remove(b)}>
                          {t.remove}
                        </Button>
                      )}
                    </Td>
                  </Tr>
                );
              })}
            </tbody>
          </Table>
        </div>
      )}
      <GrantDialog
        open={granting}
        onOpenChange={setGranting}
        nodeId={nodeId}
        labels={labels}
        fail={fail}
        onGranted={() => {
          setGranting(false);
          toast.success(t.granted);
          void load();
        }}
      />
    </Panel>
  );
}

function GrantDialog({
  open,
  onOpenChange,
  nodeId,
  labels,
  fail,
  onGranted,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  nodeId: string;
  labels: CatalogLabels;
  fail: (e: unknown) => string;
  onGranted: () => void;
}) {
  const { common } = useUiText();
  const t = labels.access;
  const [kind, setKind] = useState<"user" | "group">("user");
  const [subject, setSubject] = useState("");
  const superadmin = !!useSession().session.user?.is_superadmin;
  const [users, setUsers] = useState<User[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);

  useEffect(() => {
    if (!open || !superadmin) return;
    let live = true;
    Promise.all([apiGet<Page<User>>("/v1/users?limit=200"), apiGet<Page<Group>>("/v1/groups?limit=200")])
      .then(([u, g]) => {
        if (!live) return;
        setUsers(u.items);
        setGroups(g.items);
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, [open, superadmin]);

  const subjects =
    kind === "user"
      ? users.map((u) => ({ value: u.email, label: u.email, detail: u.display_name, keywords: [u.display_name] }))
      : groups.map((g) => ({ value: g.name, label: g.name }));
  const [role, setRole] = useState<Role>("viewer");
  const [note, setNote] = useState<Note>(null);

  const grant = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    try {
      await apiSend("POST", `/v1/catalog/nodes/${nodeId}/bindings`, { subject_kind: kind, subject, role });
      setSubject("");
      onGranted();
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
      title={t.add}
      description={t.lead}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)}>{common.cancel}</Button>
          <Button type="submit" form="grant-role" variant="primary">
            {t.grant}
          </Button>
        </>
      }
    >
      <form id="grant-role" className="grid gap-4" onSubmit={grant}>
        <Field label={t.kind}>
          {(p) => (
            <Select {...p} value={kind} onChange={(e) => setKind(e.target.value as "user" | "group")}>
              <option value="user">{t.user}</option>
              <option value="group">{t.group}</option>
            </Select>
          )}
        </Field>
        <Field label={t.subject}>
          {(p) => (
            superadmin ? (
              <Combobox
                {...p}
                required
                allowCustom
                inputType={kind === "user" ? "email" : "text"}
                value={subject}
                placeholder={t.placeholder}
                onChange={setSubject}
                options={subjects}
              />
            ) : (
              <Input
                {...p}
                required
                type={kind === "user" ? "email" : "text"}
                value={subject}
                placeholder={t.placeholder}
                onChange={(e) => setSubject(e.target.value)}
              />
            )
          )}
        </Field>
        <Field label={t.role}>
          {(p) => (
            <Select {...p} value={role} onChange={(e) => setRole(e.target.value as Role)}>
              {ROLES.map((r) => (
                <option key={r} value={r}>
                  {labels.roles[r]}
                </option>
              ))}
            </Select>
          )}
        </Field>
        <Message note={note} />
      </form>
    </Dialog>
  );
}
