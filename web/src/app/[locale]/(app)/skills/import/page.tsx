import { notFound } from "next/navigation";
import { Suspense } from "react";

import { RequireAuth } from "@/components/RequireAuth";
import { SkillsGate } from "@/components/skills/SkillsGate";
import { ImportPage } from "@/components/skills/ImportPage";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function SkillImportPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <SkillsGate title={m.skills.title} disabled={m.skills.disabled}>
        <Suspense fallback={<SkeletonRows rows={4} label={m.guard.loading} />}>
          <ImportPage locale={locale} labels={m.skills} />
        </Suspense>
      </SkillsGate>
    </RequireAuth>
  );
}
