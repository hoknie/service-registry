import { notFound } from "next/navigation";
import { Suspense } from "react";

import { RequireAuth } from "@/components/RequireAuth";
import { SkillsGate } from "@/components/skills/SkillsGate";
import { SkillsBrowser } from "@/components/skills/SkillsBrowser";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function SkillsPage({
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
          <SkillsBrowser locale={locale} labels={m.skills} access={{ access: m.catalog.access, roles: m.catalog.roles }} />
        </Suspense>
      </SkillsGate>
    </RequireAuth>
  );
}
