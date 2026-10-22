package main

type rule struct {
	message string
	broken  func(from, to string) bool
}

var rules = []rule{
	{"pkg/ must not import internal/", func(from, to string) bool {
		return under(from, "pkg") && under(to, "internal")
	}},
	{"platform must not import features, presentation or app", func(from, to string) bool {
		return under(from, "internal/platform") &&
			(under(to, "internal/feature") || under(to, "internal/presentation") || under(to, "internal/app"))
	}},
	{"unknown feature (add it to the graph)", func(from, _ string) bool {
		_, known := featureGraph[feature(from)]
		return feature(from) != "" && !known
	}},
	{"features must not import presentation, app or Fiber", func(from, to string) bool {
		return feature(from) != "" && (under(to, "internal/presentation") || under(to, "internal/app") || fiber(to))
	}},
	{"a feature may import only the features below it in the graph", func(from, to string) bool {
		f, g := feature(from), feature(to)
		return f != "" && g != "" && f != g && !allowed(f, g)
	}},
	{"features must not import adapters of other features", func(from, to string) bool {
		f, g := feature(from), feature(to)
		return f != "" && g != "" && f != g && kindOf(to) == kindAdapter
	}},
	{"a feature must not import its own adapters (app wires them)", func(from, to string) bool {
		return sameFeature(from, to) && kindOf(from) != kindAdapter && kindOf(to) == kindAdapter
	}},
	{"the domain of a feature must not import its ports, services or repositories", func(from, to string) bool {
		k := kindOf(to)
		return sameFeature(from, to) && kindOf(from) == kindTypes && (k == kindPorts || k == kindServices || k == kindRepositories)
	}},
	{"ports of a feature must not import its services or repositories", func(from, to string) bool {
		k := kindOf(to)
		return sameFeature(from, to) && kindOf(from) == kindPorts && (k == kindServices || k == kindRepositories)
	}},
	{"repositories must not import the services", func(from, to string) bool {
		return sameFeature(from, to) && kindOf(from) == kindRepositories && kindOf(to) == kindServices
	}},
	{"a repository is used only by the services of its feature", func(from, to string) bool {
		k := kindOf(from)
		return kindOf(to) == kindRepositories && !(sameFeature(from, to) && (k == kindServices || k == kindRepositories))
	}},
	{"handlers, requests, responses, middleware and mcp are for the HTTP layer only", func(from, to string) bool {
		return httpPart(to) && !under(from, "internal/presentation/http")
	}},
	{"only the console may import app", func(from, to string) bool {
		return under(from, "internal/presentation") && !under(from, "internal/presentation/console") && under(to, "internal/app")
	}},
	{"presentation must not open the database, hash passwords or seal secrets (app does)", func(from, to string) bool {
		return under(from, "internal/presentation") &&
			(under(to, "internal/platform/postgres") || under(to, "pkg/pgtx") || under(to, "internal/platform/auth") ||
				under(to, "pkg/password") || under(to, "pkg/secretbox"))
	}},
	{"presentation must not import adapters of features", func(from, to string) bool {
		return under(from, "internal/presentation") && kindOf(to) == kindAdapter
	}},
	{"Fiber is allowed only in presentation/http", func(from, to string) bool {
		return fiber(to) && !under(from, "internal/presentation/http") && !under(from, "internal/app") && !under(from, "tests")
	}},
}

var productionRules = []rule{
	{"pkg/testing is for tests only (test files, package tests, internal/testsupport)", func(from, to string) bool {
		return under(to, "pkg/testing") && !under(from, "tests") && !under(from, "internal/testsupport")
	}},
}

func sameFeature(from, to string) bool { return feature(from) != "" && feature(from) == feature(to) }

func check(from, to string, test bool) []string {
	applied := rules
	if !test {
		applied = append(append([]rule{}, rules...), productionRules...)
	}
	var problems []string
	for _, r := range applied {
		if r.broken(from, to) {
			problems = append(problems, r.message+": "+from+" -> "+to)
		}
	}
	return problems
}
