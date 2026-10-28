package types

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

const (
	FromSkill        = "skill"
	FromOrganization = "organization"
	FromDefault      = "default"
	DefaultRange     = "*"
	maxExcluded      = 50
)

type Policy struct {
	Range      *string
	Prerelease *bool
	Exclude    *[]string
}

type Effective struct {
	Range      string
	Prerelease bool
	Exclude    []string
	From       map[string]string
	constraint *semver.Constraints
}

func ValidatePolicy(p Policy) (Policy, error) {
	if p.Range != nil {
		r := strings.TrimSpace(*p.Range)
		if r == "" || len(r) > 200 {
			return Policy{}, InvalidRange
		}
		if _, err := semver.NewConstraint(r); err != nil {
			return Policy{}, InvalidRange
		}
		p.Range = &r
	}
	if p.Exclude != nil {
		if len(*p.Exclude) > maxExcluded {
			return Policy{}, InvalidPolicy
		}
		out := make([]string, 0, len(*p.Exclude))
		for _, raw := range *p.Exclude {
			v, err := ParseVersion(raw)
			if err != nil {
				return Policy{}, err
			}
			if !contains(out, v) {
				out = append(out, v)
			}
		}
		p.Exclude = &out
	}
	return p, nil
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func Merge(skill, org *Policy) Effective {
	e := Effective{Range: DefaultRange, Exclude: []string{},
		From: map[string]string{"range": FromDefault, "prerelease": FromDefault, "exclude": FromDefault}}
	for _, layer := range []struct {
		p    *Policy
		from string
	}{{org, FromOrganization}, {skill, FromSkill}} {
		if layer.p == nil {
			continue
		}
		if layer.p.Range != nil {
			e.Range, e.From["range"] = *layer.p.Range, layer.from
		}
		if layer.p.Prerelease != nil {
			e.Prerelease, e.From["prerelease"] = *layer.p.Prerelease, layer.from
		}
		if layer.p.Exclude != nil {
			e.Exclude, e.From["exclude"] = *layer.p.Exclude, layer.from
		}
	}
	return e
}

func (e *Effective) satisfied(v *semver.Version) bool {
	if e.constraint == nil {
		c, err := semver.NewConstraint(e.Range)
		if err != nil {
			c, _ = semver.NewConstraint(DefaultRange)
		}
		c.IncludePrerelease = true
		e.constraint = c
	}
	return e.constraint.Check(v)
}

func (e *Effective) Allows(version string, yanked bool) bool {
	v, err := semver.StrictNewVersion(version)
	if err != nil || yanked || contains(e.Exclude, version) {
		return false
	}
	if v.Prerelease() != "" && !e.Prerelease {
		return false
	}
	return e.satisfied(v)
}

func (e *Effective) Mark(versions []Version, published bool) []Version {
	for i := range versions {
		versions[i].Available = published && e.Allows(versions[i].Version, versions[i].Yanked)
		versions[i].Prerelease = IsPrerelease(versions[i].Version)
	}
	SortDesc(versions)
	return versions
}

func Pick(versions []Version, selector string, all bool) (*Version, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" || selector == "latest" {
		for i := range versions {
			if versions[i].Available {
				return &versions[i], nil
			}
		}
		if all && len(versions) > 0 {
			return &versions[0], nil
		}
		return nil, nil
	}
	if exact, err := ParseVersion(selector); err == nil {
		for i := range versions {
			if versions[i].Version == exact && (all || versions[i].Available) {
				return &versions[i], nil
			}
		}
		return nil, nil
	}
	c, err := semver.NewConstraint(selector)
	if err != nil {
		return nil, InvalidVersion
	}
	c.IncludePrerelease = true
	for i := range versions {
		v, err := semver.StrictNewVersion(versions[i].Version)
		if err == nil && (all || versions[i].Available) && c.Check(v) {
			return &versions[i], nil
		}
	}
	return nil, nil
}
