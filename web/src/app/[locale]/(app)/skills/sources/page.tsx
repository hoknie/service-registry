import { notFound } from "next/navigation";
import { Suspense } from "react";

import { RequireAuth } from "@/components/RequireAuth";
import { SkillsGate } from "@/components/skills/SkillsGate";
import { SourcesRedirect } from "@/components/skills/SourcesRedirect";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function SkillSourcesPage({
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
          <SourcesRedirect locale={locale} />
        </Suspense>
      </SkillsGate>
    </RequireAuth>
  );
}
