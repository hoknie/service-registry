import { notFound } from "next/navigation";

import { AdminIndex } from "@/components/admin/AdminIndex";
import { isLocale } from "@/i18n/config";

export default async function AdminPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  return <AdminIndex locale={locale} />;
}
