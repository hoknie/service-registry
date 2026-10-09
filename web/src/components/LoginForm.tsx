"use client";

import { Eye, EyeOff, LoaderCircle, LogIn } from "lucide-react";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, oauthStartUrl, type Providers, type User } from "@/lib/api";
import { safeNext } from "@/lib/next";

import { useSession } from "./SessionProvider";
import { useUiText } from "./UiText";
import { Button } from "./ui/Button";
import { Field, Input } from "./ui/Field";
import { Message } from "./ui/Message";

type Props = { locale: Locale; labels: Messages["login"] };

export function LoginForm({ locale, labels }: Props) {
  const { errors } = useUiText();
  const { session, signedIn } = useSession();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [reveal, setReveal] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [providers, setProviders] = useState<Providers | null>(null);
  const [next, setNext] = useState(`/${locale}`);

  useEffect(() => {
    const search = window.location.search;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setNext(safeNext(search, `/${locale}`));
    const code = new URLSearchParams(search).get("error");
    if (code) setError(errorText(errors, code));
    let alive = true;
    apiGet<Providers>("/v1/providers")
      .then((p) => alive && setProviders(p))
      .catch(() => alive && setProviders({ providers: [], password_login: "all" }));
    return () => {
      alive = false;
    };
  }, [locale, errors]);

  useEffect(() => {
    if (session.status === "signed-in") {
      router.replace(safeNext(window.location.search, `/${locale}`));
    }
  }, [session.status, router, locale]);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const user = await apiSend<User>("POST", "/v1/auth/login", { email, password });
      signedIn(user);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
      setBusy(false);
    }
  };

  const passwordLogin = providers?.password_login ?? "all";
  const providerButtons = providers && providers.providers.length > 0 && (
    <div className="grid gap-2">
      {providers.providers.map((p) => (
        <Button key={p.key} asChild variant="secondary" className="h-10 w-full justify-center">
          <a href={oauthStartUrl(p.key, { next })}>
            <LogIn aria-hidden="true" />
            {format(labels.signInWith, { name: p.display_name })}
          </a>
        </Button>
      ))}
    </div>
  );
  const errorNote = error && (
    <div id="login-error">
      <Message note={{ kind: "error", text: error }} />
    </div>
  );

  if (passwordLogin === "off") {
    return (
      <div className="grid gap-4">
        {errorNote}
        {providerButtons}
      </div>
    );
  }

  return (
    <div className="grid gap-5">
      <form className="grid gap-4" onSubmit={submit} aria-describedby={error ? "login-error" : undefined}>
        {passwordLogin === "superadmins" && <p className="text-sm text-muted">{labels.superadminsOnly}</p>}
        <Field label={labels.email}>
          {(p) => (
            <Input
              {...p}
              type="email"
              name="email"
              autoComplete="email"
              required
              autoFocus
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          )}
        </Field>
        <Field label={labels.password}>
          {(p) => (
            <span className="relative block">
              <Input
                {...p}
                type={reveal ? "text" : "password"}
                name="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="pr-10"
              />
              <button
                type="button"
                onClick={() => setReveal(!reveal)}
                aria-pressed={reveal}
                aria-label={labels.showPassword}
                className="absolute top-1/2 right-1.5 grid size-7 -translate-y-1/2 place-items-center rounded text-muted hover:text-ink"
              >
                {reveal ? <EyeOff aria-hidden="true" className="size-4" /> : <Eye aria-hidden="true" className="size-4" />}
              </button>
            </span>
          )}
        </Field>
        {errorNote}
        <Button type="submit" variant="primary" disabled={busy} className="mt-1 h-10 w-full justify-center">
          {busy && <LoaderCircle aria-hidden="true" className="animate-spin" />}
          {busy ? labels.submitting : labels.submit}
        </Button>
      </form>
      {providerButtons && (
        <>
          <p className="flex items-center gap-3 text-xs text-muted before:h-px before:flex-1 before:bg-line after:h-px after:flex-1 after:bg-line">
            {labels.or}
          </p>
          {providerButtons}
        </>
      )}
    </div>
  );
}
