import { Signpost } from "lucide-react";
import Link from "next/link";
import { notFound } from "next/navigation";

import { TrackField } from "@/components/shell/TrackField";
import { Button } from "@/components/ui/Button";
import { isLocale } from "@/i18n/config";
import { getMessages } from "@/i18n/messages";

export default async function NotFoundPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  const m = getMessages(locale);
  return (
    <div className="relative flex flex-1 items-center justify-center py-16">
      <TrackField className="pointer-events-none absolute inset-0 h-full w-full text-line-strong" />
      <section className="relative max-w-md text-center">
        <span className="mx-auto mb-5 grid size-14 place-items-center rounded-full border border-line bg-surface text-amber">
          <Signpost aria-hidden="true" className="size-6" />
        </span>
        <p className="font-mono text-sm text-muted">404</p>
        <h1 className="mt-1 text-3xl">{m.notFound.title}</h1>
        <p className="mt-3 text-muted">{m.notFound.text}</p>
        <Button asChild variant="primary" className="mt-6">
          <Link href={`/${locale}`}>{m.notFound.home}</Link>
        </Button>
      </section>
    </div>
  );
}
