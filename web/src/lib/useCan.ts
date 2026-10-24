"use client";

import { useSession } from "@/components/SessionProvider";

import { can, type RightAction, type RightSection } from "./rights";

export function useCan(): (section: RightSection, action: RightAction) => boolean {
  const { session } = useSession();
  const user = session.status === "signed-in" ? session.user : null;
  return (section, action) => can(user, section, action);
}

export function useSuperadmin(): boolean {
  const { session } = useSession();
  return session.status === "signed-in" && session.user.is_superadmin;
}
