import { notFound } from "next/navigation";
import { Suspense } from "react";

import { McpGate } from "@/components/mcp/McpGate";
import { ServerPage } from "@/components/mcp/ServerPage";
import { RequireAuth } from "@/components/RequireAuth";
import { SkeletonRows } from "@/components/ui/Skeleton";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function McpServerPage({
  params,
}: {
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <McpGate title={m.mcp.title} disabled={m.mcp.disabled}>
        <Suspense fallback={<SkeletonRows rows={4} label={m.guard.loading} />}>
          <ServerPage locale={locale} labels={m.mcp} />
        </Suspense>
      </McpGate>
    </RequireAuth>
  );
}
