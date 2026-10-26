import { notFound } from "next/navigation";

import { PublicFrame } from "@/components/docs/PublicFrame";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function PublicDocsPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <PublicFrame
      locale={locale}
      labels={{ space: m.docSpace, catalog: { ...m.catalog, scans: m.scans, notify: m.notify } }}
      text={m.publicDocs}
      theme={m.header.theme}
      language={m.header.language}
      skip={m.header.skip}
    />
  );
}
