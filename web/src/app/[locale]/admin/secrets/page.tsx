import { notFound } from "next/navigation";

import { GlobalSecrets } from "@/components/secrets/GlobalSecrets";
import { Crumbs } from "@/components/shell/crumbs";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function SecretsPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.admin.menu.globalSecrets }]} />
      <GlobalSecrets locale={locale} />
    </>
  );
}
