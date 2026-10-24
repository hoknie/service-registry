"use client";

import type { Locale } from "@/i18n/config";
import { useCan } from "@/lib/useCan";

import { SecretsTable } from "./SecretsTable";

export function GlobalSecrets({ locale }: { locale: Locale }) {
  const can = useCan();
  return (
    <SecretsTable
      locale={locale}
      nodeId={null}
      canManage={{ create: can("secrets", "create"), update: can("secrets", "update"), delete: can("secrets", "delete") }}
    />
  );
}
