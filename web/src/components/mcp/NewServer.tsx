"use client";

import { useRouter, useSearchParams } from "next/navigation";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";

import { Crumbs } from "../shell/crumbs";
import { Panel } from "../ui/Panel";
import { ServerForm } from "./ServerForm";

export function NewServer({
  locale,
  labels: t,
}: {
  locale: Locale;
  labels: Messages["mcp"];
}) {
  const params = useSearchParams();
  const router = useRouter();
  const folder = params.get("folder");
  const space = params.get("space");
  const back = folder
    ? `/${locale}/mcp?folder=${folder}`
    : `/${locale}/mcp${space ? `?space=${space}` : ""}`;
  return (
    <>
      <Crumbs
        items={[{ label: t.title, href: back }, { label: t.form.newTitle }]}
      />
      <Panel title={t.form.newTitle} description={t.form.newLead}>
        <ServerForm
          folder={folder}
          space={space}
          locale={locale}
          labels={t}
          onSaved={(s) => router.push(`/${locale}/mcp/server?id=${s.id}`)}
        />
      </Panel>
    </>
  );
}
