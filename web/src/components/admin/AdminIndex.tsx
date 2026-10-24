"use client";

import type { Locale } from "@/i18n/config";
import { adminHref, visibleAdminGroups } from "@/lib/admin";
import { sectionVisible } from "@/lib/rights";

import { Redirect } from "../Redirect";
import { useSession } from "../SessionProvider";

export function AdminIndex({ locale }: { locale: Locale }) {
  const { session } = useSession();
  const first = visibleAdminGroups((s) => sectionVisible(session.status === "signed-in" ? session.user : null, s))[0]?.sections[0];
  return first ? <Redirect to={adminHref(locale, first.path)} /> : null;
}
