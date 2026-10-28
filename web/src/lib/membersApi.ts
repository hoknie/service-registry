import { apiGet, apiSend } from "./api";
import { membersQuery, type GroupAccess, type Invitation, type InvitationView, type IssuedInvitation, type MemberPage, type MemberPerson, type MemberRole, type OrgMember } from "./members";

const base = (org: string) => `/v1/catalog/nodes/${encodeURIComponent(org)}` as const;

export const loadMembers = (org: string, q: string, cursor: string | null) =>
  apiGet<MemberPage>(`${base(org)}/members${membersQuery(q, cursor)}` as `/${string}`);

export const loadGroupsWithAccess = (org: string) => apiGet<{ items: GroupAccess[] }>(`${base(org)}/members/via-groups`);

export const memberCandidates = (org: string, q: string, signal?: AbortSignal) =>
  apiGet<{ items: MemberPerson[] }>(`${base(org)}/members/candidates?q=${encodeURIComponent(q)}` as `/${string}`, signal);

export const addMember = (org: string, body: { user_id?: string; email?: string; role: MemberRole }) =>
  apiSend<OrgMember>("POST", `${base(org)}/members`, body);

export const setMemberRole = (org: string, user: string, role: MemberRole | null) =>
  apiSend<OrgMember>("PATCH", `${base(org)}/members/${encodeURIComponent(user)}` as `/${string}`, { role });

export const removeMember = (org: string, user: string) => apiSend("DELETE", `${base(org)}/members/${encodeURIComponent(user)}` as `/${string}`);

export const loadInvitations = (org: string) => apiGet<{ items: Invitation[] }>(`${base(org)}/invitations`);

export const invite = (org: string, body: { email: string; role: MemberRole; locale: string }) =>
  apiSend<IssuedInvitation>("POST", `${base(org)}/invitations`, body);

export const resendInvitation = (org: string, id: string) =>
  apiSend<IssuedInvitation>("POST", `${base(org)}/invitations/${encodeURIComponent(id)}/resend` as `/${string}`, {});

export const revokeInvitation = (org: string, id: string) => apiSend("DELETE", `${base(org)}/invitations/${encodeURIComponent(id)}` as `/${string}`);

const invitationPath = (token: string) => `/v1/invitations/${encodeURIComponent(token)}` as `/${string}`;

export const viewInvitation = (token: string) => apiGet<InvitationView>(invitationPath(token));

export const acceptInvitation = (token: string) =>
  apiSend<{ organization: { id: string; name: string } }>("POST", `${invitationPath(token)}/accept` as `/${string}`, {});

export const registerInvited = (token: string, name: string, password: string) =>
  apiSend("POST", `${invitationPath(token)}/register` as `/${string}`, { name, password });
