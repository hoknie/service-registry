import { notFound } from "next/navigation";
import { Suspense } from "react";

import { ScansAdmin } from "@/components/admin/ScansAdmin";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function ScansPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard} superadmin>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.scans }]} />
      <Suspense fallback={<SkeletonTable rows={6} label={m.guard.loading} />}>
        <ScansAdmin locale={locale} labels={m.scans} />
      </Suspense>
    </RequireAuth>
  );
}
