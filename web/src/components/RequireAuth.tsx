"use client";

import { ShieldAlert } from "lucide-react";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";

import { useSession } from "./SessionProvider";
import { EmptyState } from "./ui/EmptyState";
import { Skeleton } from "./ui/Skeleton";

type Props = {
  locale: Locale;
  labels: Messages["guard"];
  superadmin?: boolean;
  children: ReactNode;
};

export function RequireAuth({ locale, labels, superadmin = false, children }: Props) {
  const { session } = useSession();
  const router = useRouter();
  const pathname = usePathname() ?? `/${locale}`;

  useEffect(() => {
    if (session.status === "anonymous") {
      router.replace(`/${locale}/login?next=${encodeURIComponent(pathname)}`);
    }
  }, [session.status, router, locale, pathname]);

  if (session.status !== "signed-in") {
    return (
      <div role="status" className="grid gap-4">
        <span className="sr-only">{labels.loading}</span>
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-96 max-w-full" />
        <div className="mt-4 grid gap-4 sm:grid-cols-3">
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
        </div>
      </div>
    );
  }
  if (superadmin && !session.user.is_superadmin) {
    return (
      <div role="alert">
        <EmptyState icon={ShieldAlert} title={labels.forbidden} className="mx-auto mt-8 max-w-lg" />
      </div>
    );
  }
  return <>{children}</>;
}
