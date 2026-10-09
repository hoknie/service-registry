"use client";

import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { when } from "@/i18n/time";
import { apiGet, type Identity, type Items } from "@/lib/api";

import { useUiText } from "../UiText";
import { Message } from "../ui/Message";
import { SkeletonTable } from "../ui/Skeleton";

type Props = {
  userId: string;
  locale: Locale;
  labels: Messages["admin"]["users"]["identities"];
  fail: (e: unknown) => string;
};

export function UserIdentities({ userId, locale, labels, fail }: Props) {
  const { common } = useUiText();
  const [items, setItems] = useState<Identity[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    apiGet<Items<Identity>>(`/v1/users/${userId}/identities`)
      .then((r) => alive && setItems(r.items))
      .catch((e) => alive && setError(fail(e)));
    return () => {
      alive = false;
    };
  }, [userId, fail]);

  return (
    <section className="mt-6 grid gap-3 border-t border-line pt-5">
      <h3>{labels.title}</h3>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!items && !error && <SkeletonTable rows={1} label={common.loading} />}
      {items && items.length === 0 && <p className="text-sm text-muted">{labels.empty}</p>}
      {items && items.length > 0 && (
        <ul className="divide-y divide-line rounded-lg border border-line">
          {items.map((i) => (
            <li key={i.id} className="px-3 py-2 text-sm">
              <span className="block truncate font-medium text-ink">{i.display_name}</span>
              <span className="block truncate text-xs text-muted">
                {i.email ?? "—"} · {labels.lastLogin}: {when(i.last_login_at, locale, "—")}
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
