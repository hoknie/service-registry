"use client";

import { Plug } from "lucide-react";
import type { ReactNode } from "react";

import { useSession } from "../SessionProvider";
import { EmptyState } from "../ui/EmptyState";

export function McpGate({
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
  if (session.user.features?.mcp_gateway !== true)
    return <EmptyState icon={Plug} title={title} text={disabled} />;
  return <>{children}</>;
}
