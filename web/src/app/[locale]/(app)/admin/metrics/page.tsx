import { notFound } from "next/navigation";
import { Suspense } from "react";

import { MetricsAdmin } from "@/components/admin/MetricsAdmin";
import { Crumbs } from "@/components/shell/crumbs";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function MetricsPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.metrics }]} />
      <Suspense fallback={<SkeletonTable rows={6} label={m.guard.loading} />}>
        <MetricsAdmin locale={locale} labels={m.admin.metrics} />
      </Suspense>
    </>
  );
}
