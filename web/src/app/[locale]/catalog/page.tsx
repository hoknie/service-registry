import { notFound } from "next/navigation";
import { Suspense } from "react";

import { CatalogBrowser } from "@/components/catalog/CatalogBrowser";
import { RequireAuth } from "@/components/RequireAuth";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function CatalogPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <Suspense fallback={<SkeletonRows rows={6} label={m.catalog.loading} />}>
        <CatalogBrowser locale={locale} labels={m.catalog} />
      </Suspense>
    </RequireAuth>
  );
}
