import { notFound } from "next/navigation";

import { LoginForm } from "@/components/LoginForm";
import { BrandMark } from "@/components/shell/BrandMark";
import { TrackField } from "@/components/shell/TrackField";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function LoginPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <div className="relative flex flex-1 items-center justify-center py-10">
      <TrackField className="pointer-events-none absolute inset-0 -z-0 h-full w-full text-line-strong" />
      <section className="relative w-full max-w-sm rounded-xl border border-line bg-surface p-7 shadow-pop sm:p-8">
        <BrandMark className="mb-6 size-10" />
        <h1 className="text-2xl">{m.login.title}</h1>
        <p className="mt-1 mb-6 text-sm text-muted">{m.login.lead}</p>
        <LoginForm locale={locale} labels={m.login} />
      </section>
    </div>
  );
}
