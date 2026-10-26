import { FileSearch } from "lucide-react";
import { notFound } from "next/navigation";
import { Suspense } from "react";

import { SearchView } from "@/components/knowledge/SearchView";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { PageHeader } from "@/components/ui/Panel";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function SearchPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <Crumbs items={[{ label: m.search.title }]} />
      <PageHeader glyph={FileSearch} title={m.search.title} lead={m.search.lead} />
      <Suspense fallback={<SkeletonRows rows={4} label={m.guard.loading} />}>
        <SearchView locale={locale} labels={m.search} kinds={m.catalog.docs.kinds} />
      </Suspense>
    </RequireAuth>
  );
}
