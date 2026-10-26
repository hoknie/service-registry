import { notFound } from "next/navigation";
import type { ReactNode } from "react";

import { SessionProvider } from "@/components/SessionProvider";
import { AppShell } from "@/components/shell/AppShell";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function AppLayout({ children, params }: { children: ReactNode; params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <SessionProvider>
      <AppShell locale={locale} labels={{ header: m.header, palette: m.palette }}>
        {children}
      </AppShell>
    </SessionProvider>
  );
}
