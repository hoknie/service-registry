import { notFound } from "next/navigation";
import { Suspense } from "react";

import { GroupPage } from "@/components/admin/GroupPage";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function GroupAdminPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard} superadmin>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.groups, href: `/${locale}/admin/groups` }, { label: m.admin.groups.page.crumb }]} />
      <Suspense fallback={<SkeletonRows rows={4} label={m.guard.loading} />}>
        <GroupPage locale={locale} labels={m.admin} />
      </Suspense>
    </RequireAuth>
  );
}
