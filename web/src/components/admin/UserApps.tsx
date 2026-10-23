"use client";

import type { Locale } from "@/i18n/config";

import { AppList } from "../account/AppList";
import type { TokenLabels } from "../account/TokenTable";

type Props = { userId: string; locale: Locale; labels: TokenLabels; fail: (e: unknown) => string };

export function UserApps({ userId, locale, labels, fail }: Props) {
  return (
    <section className="grid gap-3">
      <h3>{labels.apps.title}</h3>
      <div className="overflow-hidden rounded-md border border-line">
        <AppList endpoint={`/v1/users/${userId}/apps`} locale={locale} labels={labels} empty={labels.apps.userEmpty} fail={fail} />
      </div>
    </section>
  );
}
