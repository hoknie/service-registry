package http

import "svc-registry/internal/presentation/http/handlers"

func (r routes) members(api *handlers.Handlers) {
	r.add("/api/v1/catalog/nodes/:id/members", true, get(api.ListOrgMembers), post(api.AddOrgMember))
	r.add("/api/v1/catalog/nodes/:id/members/candidates", true, get(api.OrgMemberCandidates))
	r.add("/api/v1/catalog/nodes/:id/members/via-groups", true, get(api.OrgMembersViaGroups))
	r.add("/api/v1/catalog/nodes/:id/members/:user_id", true, patch(api.SetOrgMemberRole), del(api.RemoveOrgMember))
	r.add("/api/v1/catalog/nodes/:id/invitations", true, get(api.ListInvitations), post(api.CreateInvitation))
	r.add("/api/v1/catalog/nodes/:id/invitations/:iid", true, del(api.RevokeInvitation))
	r.add("/api/v1/catalog/nodes/:id/invitations/:iid/resend", true, post(api.ResendInvitation))
	r.add("/api/v1/invitations/:token", false, get(api.ViewInvitation))
	r.add("/api/v1/invitations/:token/accept", true, post(api.AcceptInvitation))
	r.add("/api/v1/invitations/:token/register", true, post(api.RegisterInvited))
}
