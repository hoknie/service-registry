import { UserRound } from "lucide-react";
import { notFound } from "next/navigation";
import { Suspense } from "react";

import { AccountView } from "@/components/AccountView";
import { RequireAuth } from "@/components/RequireAuth";
import { Crumbs } from "@/components/shell/crumbs";
import { PageHeader } from "@/components/ui/Panel";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function AccountPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <Crumbs items={[{ label: m.account.title }]} />
      <div className="mx-auto max-w-5xl">
        <PageHeader glyph={UserRound} title={m.account.title} lead={m.account.lead} />
        <Suspense fallback={<SkeletonRows rows={4} label={m.guard.loading} />}>
          <AccountView locale={locale} labels={m.account} tokenLabels={m.tokens} superadminLabel={m.header.user.superadmin} />
        </Suspense>
      </div>
    </RequireAuth>
  );
}
