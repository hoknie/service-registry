import { notFound } from "next/navigation";
import { Suspense } from "react";

import { CreateNodePage } from "@/components/catalog/CreateNodePage";
import { RequireAuth } from "@/components/RequireAuth";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function NewNodePage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <Suspense fallback={<SkeletonRows rows={6} label={m.catalog.loading} />}>
        <CreateNodePage locale={locale} labels={{ ...m.catalog, scans: m.scans, notify: m.notify }} />
      </Suspense>
    </RequireAuth>
  );
}
