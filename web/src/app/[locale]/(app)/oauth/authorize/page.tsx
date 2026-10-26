import { notFound } from "next/navigation";

import { ConsentView } from "@/components/oauth/ConsentView";
import { RequireAuth } from "@/components/RequireAuth";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function OAuthAuthorizePage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <RequireAuth locale={locale} labels={m.guard}>
      <div className="mx-auto w-full max-w-lg py-6">
        <ConsentView labels={m.oauth} scopeLabels={m.tokens.dialog} />
      </div>
    </RequireAuth>
  );
}
