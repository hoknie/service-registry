import { notFound } from "next/navigation";
import { Suspense } from "react";

import { UserPage } from "@/components/admin/UserPage";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function UserAdminPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard} superadmin>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.users, href: `/${locale}/admin/users` }, { label: m.admin.users.page.crumb }]} />
      <Suspense fallback={<SkeletonRows rows={4} label={m.guard.loading} />}>
        <UserPage locale={locale} labels={m.admin} tokenLabels={m.tokens} />
      </Suspense>
    </RequireAuth>
  );
}
