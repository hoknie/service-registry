import { notFound } from "next/navigation";

import { Redirect } from "@/components/Redirect";
import { isLocale } from "@/i18n/config";
import { ADMIN_PATHS, adminHref } from "@/lib/admin";

export default async function AdminPage({ params }: { params: Promise<{ locale: string }> }) {
  const { locale } = await params;
  if (!isLocale(locale)) notFound();
  return <Redirect to={adminHref(locale, ADMIN_PATHS[0] ?? "users")} />;
}
