import { notFound } from "next/navigation";

import { ClustersAdmin } from "@/components/deploy/ClustersAdmin";
import { Crumbs } from "@/components/shell/crumbs";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function ClustersPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <>
      <Crumbs items={[{ label: m.header.nav.admin }, { label: m.header.nav.clusters }]} />
      <ClustersAdmin locale={locale} labels={m.admin.clusters} pager={m.admin.users.pager} />
    </>
  );
}
