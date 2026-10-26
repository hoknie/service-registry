import { notFound } from "next/navigation";

import { TokensAdmin } from "@/components/admin/TokensAdmin";
import { Crumbs } from "@/components/shell/crumbs";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function TokensPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.tokens }]} />
      <TokensAdmin locale={locale} labels={m.admin} tokenLabels={m.tokens} />
    </>
  );
}
