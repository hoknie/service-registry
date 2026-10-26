import { notFound } from "next/navigation";
import { Suspense } from "react";

import { StatsAdmin } from "@/components/admin/StatsAdmin";
import { Crumbs } from "@/components/shell/crumbs";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function StatsPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.stats }]} />
      <Suspense fallback={<SkeletonTable rows={6} label={m.guard.loading} />}>
        <StatsAdmin locale={locale} labels={m.admin.metrics} />
      </Suspense>
    </>
  );
}
