package catalog

type Role int

const (
	RoleViewer Role = iota + 1
	RoleEditor
	RoleAdmin
)

func (r Role) String() string {
	switch r {
	case RoleViewer:
		return "viewer"
	case RoleEditor:
		return "editor"
	case RoleAdmin:
		return "admin"
	}
	return "unknown"
}

func ParseRole(s string) (Role, bool) {
	switch s {
	case "viewer":
		return RoleViewer, true
	case "editor":
		return RoleEditor, true
	case "admin":
		return RoleAdmin, true
	}
	return 0, false
}

func (r Role) Allows(p Permission) bool {
	switch p {
	case PermRead:
		return r >= RoleViewer
	case PermWrite, PermKeys:
		return r >= RoleEditor
	case PermAccess:
		return r == RoleAdmin
	}
	return false
}

type Permission int

const (
	PermRead Permission = iota + 1
	PermWrite
	PermKeys
	PermAccess
)

var AllPermissions = [...]Permission{PermRead, PermWrite, PermKeys, PermAccess}

func (p Permission) String() string {
	switch p {
	case PermRead:
		return "catalog.read"
	case PermWrite:
		return "catalog.write"
	case PermKeys:
		return "catalog.keys"
	case PermAccess:
		return "catalog.access"
	}
	return "unknown"
}

type Visibility int

const (
	VisibilityNone Visibility = iota
	VisibilityNavigate
	VisibilityRead
)
