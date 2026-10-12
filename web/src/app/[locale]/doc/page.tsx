import { notFound } from "next/navigation";

import { DocSpace } from "@/components/docs/DocSpace";
import { RequireAuth } from "@/components/RequireAuth";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function DocPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <DocSpace locale={locale} labels={{ space: m.docSpace, catalog: { ...m.catalog, scans: m.scans } }} />
    </RequireAuth>
  );
}
