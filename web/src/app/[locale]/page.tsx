import { LayoutDashboard } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";

import { Dashboard } from "@/components/Dashboard";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { SignedInAs } from "@/components/SignedInAs";
import { Button } from "@/components/ui/Button";
import { PageHeader } from "@/components/ui/Panel";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function HomePage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <Crumbs items={[{ label: m.header.nav.home }]} />
      <PageHeader glyph={LayoutDashboard}
        title={m.home.title}
        lead={m.home.lead}
        actions={
          <Button asChild variant="primary">
            <Link href={`/${locale}/catalog`}>{m.home.catalogLink}</Link>
          </Button>
        }
      >
        <SignedInAs template={m.home.signedInAs} />
      </PageHeader>
      <Dashboard locale={locale} labels={m.home} />
    </RequireAuth>
  );
}
