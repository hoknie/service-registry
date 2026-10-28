"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { useEffect } from "react";

import type { Locale } from "@/i18n/config";

export function SourcesRedirect({ locale }: { locale: Locale }) {
  const router = useRouter();
  const folder = useSearchParams().get("folder");
  useEffect(() => {
    router.replace(folder ? `/${locale}/skills?folder=${encodeURIComponent(folder)}&tab=settings&section=sources` : `/${locale}/skills`);
  }, [router, folder, locale]);
  return null;
}
