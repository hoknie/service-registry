package types

import (
	"slices"
	"strings"

	"github.com/google/uuid"
)

type Group struct {
	ID           uuid.UUID
	Name         string
	Organization *NodeRef
	MemberCount  int64
	OnCallURL    *string
	Contacts     []Contact
	CreatedAt    string
	UpdatedAt    string
}

type MembershipSource string

const SourceManual MembershipSource = "manual"

func OAuthSource(provider string) MembershipSource { return MembershipSource("oauth:" + provider) }

type Member struct {
	User          UserSummary
	Source        MembershipSource
	ContactEditor bool
	EditorSource  *MembershipSource
}

type OwnGroup struct {
	Group           Group
	CanEditContacts bool
}

type MembersLock string

const (
	LockNone     MembersLock = ""
	LockRights   MembersLock = "rights"
	LockBindings MembersLock = "bindings"
)

type GroupDetails struct {
	Group            Group
	Members          []Member
	Permissions      Rights
	MembersLock      MembersLock
	CanManageMembers bool
	MembersHidden    bool
}

type GroupList struct {
	Page         Page[Group]
	Organization *NodeRef
	CanManage    bool
}

type OrganizationAccess struct {
	Org      NodeRef
	TopLevel bool
	Admin    bool
}

type ManagedGroup struct {
	Group Group
	Nodes []NodeRef
}

type MemberCandidate struct {
	ID          uuid.UUID
	DisplayName string
	Email       string
}

type Membership struct {
	GroupExists bool
	UserExists  bool
	Member      bool
	Editor      bool
}

type Section string

const (
	SectionUsers        Section = "users"
	SectionGroups       Section = "groups"
	SectionTokens       Section = "tokens"
	SectionClusters     Section = "clusters"
	SectionEnvironments Section = "environments"
	SectionLinkKinds    Section = "link_kinds"
	SectionSecrets      Section = "secrets"
	SectionScans        Section = "scans"
	SectionCatalog      Section = "catalog"
)

type Action string

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

var Actions = []Action{ActionCreate, ActionRead, ActionUpdate, ActionDelete}

var Sections = []Section{SectionUsers, SectionGroups, SectionTokens, SectionClusters, SectionEnvironments, SectionLinkKinds,
	SectionSecrets, SectionScans, SectionCatalog}

var SectionActions = map[Section][]Action{
	SectionUsers:        {ActionCreate, ActionRead, ActionUpdate},
	SectionGroups:       Actions,
	SectionTokens:       {ActionRead, ActionDelete},
	SectionClusters:     Actions,
	SectionEnvironments: {ActionCreate, ActionUpdate, ActionDelete},
	SectionLinkKinds:    {ActionCreate, ActionUpdate, ActionDelete},
	SectionSecrets:      Actions,
	SectionScans:        {ActionRead},
	SectionCatalog:      Actions,
}

var sectionScopes = map[Section][2]Scope{
	SectionUsers:        {ScopeUsersRead, ScopeUsersWrite},
	SectionGroups:       {ScopeGroupsRead, ScopeGroupsWrite},
	SectionClusters:     {ScopeClustersRead, ScopeClustersWrite},
	SectionEnvironments: {ScopeSettingsRead, ScopeSettingsWrite},
	SectionLinkKinds:    {ScopeSettingsRead, ScopeSettingsWrite},
	SectionSecrets:      {ScopeSettingsRead, ScopeSettingsWrite},
	SectionScans:        {ScopeSettingsRead, ScopeSettingsWrite},
	SectionCatalog:      {ScopeRead, ScopeCatalogRoot},
}

type Rights map[Section][]Action

func (r Rights) Has(section Section, action Action) bool {
	return slices.Contains(r[section], action)
}

func (r Rights) Any() bool {
	for _, actions := range r {
		if len(actions) > 0 {
			return true
		}
	}
	return false
}

func (r Rights) Union(other Rights) Rights {
	out := Rights{}
	for _, section := range Sections {
		for _, action := range SectionActions[section] {
			if r.Has(section, action) || other.Has(section, action) {
				out[section] = append(out[section], action)
			}
		}
	}
	return out
}

func (r Rights) Canonical() Rights { return r.Union(nil) }

func GroupNameKeys(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = strings.ToLower(n)
	}
	return out
}

func (r Rights) ForScopes(scopes Scopes) Rights {
	out := Rights{}
	for _, section := range Sections {
		pair, ok := sectionScopes[section]
		if !ok {
			continue
		}
		for _, action := range r[section] {
			if scopes.Has(pair[scopeHalf(action)]) {
				out[section] = append(out[section], action)
			}
		}
	}
	return out
}

func (r Rights) AllowsScope(scope Scope) bool {
	for section, pair := range sectionScopes {
		for _, action := range r[section] {
			if pair[scopeHalf(action)] == scope {
				return true
			}
		}
	}
	return false
}

func scopeHalf(action Action) int {
	if action == ActionRead {
		return 0
	}
	return 1
}

func FullRights() Rights {
	out := Rights{}
	for _, section := range Sections {
		out[section] = slices.Clone(SectionActions[section])
	}
	return out
}

func ValidateRights(in map[string][]string) (Rights, error) {
	out := Rights{}
	for raw, actions := range in {
		section := Section(raw)
		allowed, ok := SectionActions[section]
		if !ok {
			return nil, InvalidPermissions
		}
		seen := map[Action]bool{}
		for _, a := range actions {
			action := Action(a)
			if !slices.Contains(allowed, action) || seen[action] {
				return nil, InvalidPermissions
			}
			seen[action] = true
		}
		for _, action := range allowed {
			if seen[action] {
				out[section] = append(out[section], action)
			}
		}
	}
	return out, nil
}
