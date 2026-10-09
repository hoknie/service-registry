import { notFound } from "next/navigation";

import { EnvironmentsAdmin } from "@/components/deploy/EnvironmentsAdmin";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function EnvironmentsPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard} superadmin>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.environments }]} />
      <EnvironmentsAdmin locale={locale} labels={m.admin.environments} />
    </RequireAuth>
  );
}
