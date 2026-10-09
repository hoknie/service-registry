import "@fontsource-variable/ibm-plex-sans/wght.css";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "../globals.css";

import type { Metadata, Viewport } from "next";
import { notFound } from "next/navigation";
import type { ReactNode } from "react";

import { SessionProvider } from "@/components/SessionProvider";
import { AppShell } from "@/components/shell/AppShell";
import { UiTextProvider } from "@/components/UiText";
import { isLocale, locales } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";
import { SIDEBAR_SCRIPT } from "@/lib/sidebar";
import { THEME_SCRIPT } from "@/lib/theme";

type Props = { children: ReactNode; params: Promise<{ locale: string }> };

export function generateStaticParams() {
  return locales.map((locale) => ({ locale }));
}

export const dynamicParams = false;

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f2f4f7" },
    { media: "(prefers-color-scheme: dark)", color: "#0e131a" },
  ],
};

export async function generateMetadata({ params }: Omit<Props, "children">): Promise<Metadata> {
  const { locale } = await params;
  if (!isLocale(locale)) return {};
  const m = getMessages(locale);
  return { title: m.meta.title, description: m.meta.description };
}

export default async function LocaleLayout({ children, params }: Props) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);

  return (
    <html lang={locale} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME_SCRIPT + ";" + SIDEBAR_SCRIPT }} />
      </head>
      <body>
        <UiTextProvider value={{ common: m.common, errors: m.errors }}>
          <SessionProvider>
            <AppShell locale={locale} labels={{ header: m.header, palette: m.palette }}>
              {children}
            </AppShell>
          </SessionProvider>
        </UiTextProvider>
      </body>
    </html>
  );
}
