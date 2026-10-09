import { notFound } from "next/navigation";

import { UsersAdmin } from "@/components/admin/UsersAdmin";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function UsersPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard} superadmin>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.users }]} />
      <UsersAdmin locale={locale} labels={m.admin} />
    </RequireAuth>
  );
}
