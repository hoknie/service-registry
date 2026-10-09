"use client";

import { format } from "@/i18n/format";

import { useSession } from "./SessionProvider";

export function SignedInAs({ template }: { template: string }) {
  const { session } = useSession();
  if (session.status !== "signed-in") return null;
  return <p className="mt-2 text-sm text-muted">{format(template, { name: session.user.display_name })}</p>;
}
