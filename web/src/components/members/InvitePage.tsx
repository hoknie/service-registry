"use client";

import { LogIn } from "lucide-react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { when } from "@/i18n/time";
import { errorCode } from "@/lib/api";
import { providerStart, type InvitationView, type MemberRole } from "@/lib/members";
import { acceptInvitation, registerInvited, viewInvitation } from "@/lib/membersApi";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Skeleton } from "../ui/Skeleton";

type Props = { locale: Locale; labels: Messages["invite"]; roles: Record<string, string> };

export function InvitePage({ locale, labels: t, roles }: Props) {
  const { errors } = useUiText();
  const { session, refresh } = useSession();
  const router = useRouter();
  const [token, setToken] = useState("");
  const [view, setView] = useState<InvitationView | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const q = new URLSearchParams(window.location.search);
    const found = q.get("token") ?? "";
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setToken(found);
    const code = q.get("error");
    if (code) setError(errorText(errors, code));
    if (!found) {
      setError(t.invalid);
      return;
    }
    viewInvitation(found)
      .then(setView)
      .catch((e) => setError(errorCode(e) === "not_found" ? t.invalid : errorText(errors, errorCode(e))));
  }, [errors, t.invalid]);

  const open = useCallback(
    async (org: string) => {
      await refresh();
      router.push(`/${locale}/catalog?node=${encodeURIComponent(org)}`);
    },
    [locale, refresh, router],
  );

  const accept = async () => {
    setBusy(true);
    setError(null);
    try {
      await open((await acceptInvitation(token)).organization.id);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
      setBusy(false);
    }
  };

  const register = async (event: FormEvent) => {
    event.preventDefault();
    if (!view) return;
    setBusy(true);
    setError(null);
    try {
      await registerInvited(token, name, password);
      await open(view.organization.id);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
      setBusy(false);
    }
  };

  if (!view) {
    return (
      <div className="grid gap-3">
        <h1 className="text-2xl">{t.title}</h1>
        {error ? <Message note={{ kind: "error", text: error }} /> : <Skeleton className="h-24 w-full" />}
      </div>
    );
  }

  const here = `/${locale}/invite?token=${token}`;
  const loginHref = `/${locale}/login?next=${encodeURIComponent(here)}`;
  const providers = view.providers.length > 0 && (
    <div className="grid gap-2">
      {view.providers.map((p) => (
        <Button key={p.key} asChild variant="secondary" className="h-10 w-full justify-center">
          <a href={providerStart(p.key, token, locale)}>
            <LogIn aria-hidden="true" />
            {format(t.withProvider, { name: p.name })}
          </a>
        </Button>
      ))}
    </div>
  );
  const user = session.status === "signed-in" ? session.user : null;
  const pending = view.state === "pending";

  return (
    <div className="grid gap-5">
      <div>
        <h1 className="text-2xl">{format(t.heading, { organization: view.organization.name })}</h1>
        <p className="mt-2 text-sm text-muted">
          {format(view.invited_by ? t.leadBy : t.lead, { name: view.invited_by?.name ?? "", role: roles[view.role as MemberRole] ?? view.role })}
        </p>
        {pending && <p className="mt-1 text-xs text-muted">{format(t.until, { date: when(view.expires_at, locale, "") })}</p>}
      </div>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!pending && <Message note={{ kind: "error", text: t.states[view.state as "expired" | "accepted" | "revoked"] }} />}
      {pending && session.status === "loading" && <Skeleton className="h-10 w-full" />}
      {pending && user && user.email.toLowerCase() === view.email && (
        <Button variant="primary" className="h-10 w-full justify-center" busy={busy} onClick={accept}>
          {t.accept}
        </Button>
      )}
      {pending && user && user.email.toLowerCase() !== view.email && (
        <Message note={{ kind: "error", text: format(t.otherAccount, { email: view.email, current: user.email }) }} />
      )}
      {pending && session.status === "anonymous" && view.account === "exists" && (
        <div className="grid gap-3">
          <p className="text-sm text-ink-2">{format(t.signInFirst, { email: view.email })}</p>
          {view.password_login && (
            <Button asChild variant="primary" className="h-10 w-full justify-center">
              <a href={loginHref}>{t.signIn}</a>
            </Button>
          )}
          {providers}
        </div>
      )}
      {pending && session.status === "anonymous" && view.account === "new" && (
        <div className="grid gap-4">
          {view.password_login && (
            <form className="grid gap-4" onSubmit={register}>
              <Field label={t.email}>{(f) => <Input {...f} type="email" value={view.email} readOnly />}</Field>
              <Field label={t.name}>{(f) => <Input {...f} required autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} />}</Field>
              <Field label={t.password} hint={t.passwordHint}>
                {(f) => <Input {...f} type="password" required autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />}
              </Field>
              <Button type="submit" variant="primary" className="h-10 w-full justify-center" busy={busy}>
                {t.create}
              </Button>
            </form>
          )}
          {providers && view.password_login && <p className="text-center text-xs text-muted">{t.or}</p>}
          {providers}
        </div>
      )}
    </div>
  );
}
