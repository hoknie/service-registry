export type MemberRole = "viewer" | "editor" | "admin";
export type MemberSource = "manual" | "invitation" | "role" | "owner" | "group";
export type InvitationState = "pending" | "expired" | "accepted" | "revoked";
export type MailState = "sent" | "disabled" | "failed";

export const MEMBER_ROLES: MemberRole[] = ["viewer", "editor", "admin"];

export type MemberPerson = { id: string; name: string; email: string; service: boolean };

export type OrgMember = {
  user: MemberPerson;
  role: MemberRole | null;
  source: MemberSource;
  added_by: { id: string; name: string } | null;
  added_at: string;
};

export type MemberPage = { items: OrgMember[]; next_cursor: string | null; can_manage: boolean };

export type GroupAccess = { id: string; name: string; role: MemberRole; users: number };

export type Invitation = {
  id: string;
  email: string;
  role: MemberRole;
  locale: string;
  invited_by: { id: string; name: string } | null;
  created_at: string;
  expires_at: string;
  state: InvitationState;
};

export type IssuedInvitation = { invitation: Invitation; link: string; mail: MailState };

export type InvitationView = {
  organization: { id: string; name: string };
  email: string;
  role: MemberRole;
  invited_by: { name: string } | null;
  expires_at: string;
  state: InvitationState;
  account: "exists" | "new";
  password_login: boolean;
  providers: { key: string; name: string }[];
};

export function absoluteLink(link: string, origin: string): string {
  return link.startsWith("/") ? origin.replace(/\/$/, "") + link : link;
}

export function stateTone(state: InvitationState): "neutral" | "amber" | "signal" | "danger" {
  return state === "pending" ? "signal" : state === "expired" ? "amber" : state === "revoked" ? "danger" : "neutral";
}

export function membersQuery(q: string, cursor: string | null): string {
  const p = new URLSearchParams();
  if (q.trim()) p.set("q", q.trim());
  if (cursor) p.set("cursor", cursor);
  const s = p.toString();
  return s ? `?${s}` : "";
}

export function providerStart(key: string, token: string, locale: string): string {
  const next = `/${locale}/invite?token=${token}`;
  return `/api/v1/auth/oauth/${encodeURIComponent(key)}/start?invite=${encodeURIComponent(token)}&next=${encodeURIComponent(next)}`;
}

export function displayName(p: { name: string; email: string }): string {
  return p.name.trim() || p.email;
}
