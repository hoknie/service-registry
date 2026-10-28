"use client";

import { Sparkles } from "lucide-react";
import type { ReactNode } from "react";

import { useSession } from "../SessionProvider";
import { EmptyState } from "../ui/EmptyState";

export function SkillsGate({
  title,
  disabled,
  children,
}: {
  title: string;
  disabled: string;
  children: ReactNode;
}) {
  const { session } = useSession();
  if (session.status !== "signed-in") return null;
  if (session.user.features?.skills !== true)
    return <EmptyState icon={Sparkles} title={title} text={disabled} />;
  return <>{children}</>;
}
