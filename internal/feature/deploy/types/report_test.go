package types

import (
	"errors"
	"strings"
	"testing"
)

func TestReportValidation(t *testing.T) {
	t.Parallel()
	w := ReportWorkload{UID: "u1", Kind: KindDeployment, Namespace: "backend", Name: "api"}
	ok := Report{AgentVersion: "0.2.0", Workloads: []ReportWorkload{w}}
	if err := ValidateReport(ok); err != nil {
		t.Fatal(err)
	}
	failed := Report{AgentVersion: "0.2.0", Error: &ReportError{Code: "k8s.forbidden", Message: "no"}}
	if err := ValidateReport(failed); err != nil {
		t.Fatal(err)
	}
	with := func(f func(*Report)) Report {
		r := ok
		r.Workloads = append([]ReportWorkload(nil), ok.Workloads...)
		f(&r)
		return r
	}
	for name, r := range map[string]Report{
		"no version":     with(func(r *Report) { r.AgentVersion = "" }),
		"long version":   with(func(r *Report) { r.AgentVersion = strings.Repeat("v", 65) }),
		"many missing":   with(func(r *Report) { r.Missing = make([]string, 21) }),
		"unknown error":  with(func(r *Report) { r.Error = &ReportError{Code: "k8s.agent_silent"} }),
		"long message":   with(func(r *Report) { r.Error = &ReportError{Code: "k8s.timeout", Message: strings.Repeat("m", 1025)} }),
		"unknown kind":   with(func(r *Report) { r.Workloads[0].Kind = "Job" }),
		"no uid":         with(func(r *Report) { r.Workloads[0].UID = "" }),
		"many workloads": with(func(r *Report) { r.Workloads = make([]ReportWorkload, MaxReportWorkloads+1) }),
	} {
		if err := ValidateReport(r); !errors.Is(err, InvalidReport) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestCandidatesNeedAnAnnotationOrARuleLabel(t *testing.T) {
	t.Parallel()
	rules := []Rule{{Label: "app.kubernetes.io/name", Project: "acme/{namespace}"}}
	for name, tt := range map[string]struct {
		w    Workload
		want bool
	}{
		"annotation":          {Workload{Annotations: map[string]string{AnnotationProject: "acme/api"}}, true},
		"template annotation": {Workload{TemplateAnnotations: map[string]string{AnnotationProject: "acme/api"}}, true},
		"rule label":          {Workload{Labels: map[string]string{"app.kubernetes.io/name": "api"}}, true},
		"neither":             {Workload{Labels: map[string]string{"tier": "web"}}, false},
	} {
		if got := Candidate(tt.w, rules); got != tt.want {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestReportWorkloadsRoundTrip(t *testing.T) {
	t.Parallel()
	w := Workload{UID: "u", Kind: KindStatefulSet, Namespace: "n", Name: "db", Labels: map[string]string{"a": "b"},
		Containers: []Container{{Name: "db", Image: "pg:17"}}, Desired: 3, Ready: 2, Updated: 1,
		Conditions: []Condition{{Type: "Available", Status: "True"}}}
	back := ReportWorkloadOf(w).Workload()
	if back.UID != w.UID || back.Ready != 2 || back.Containers[0].Image != "pg:17" || back.Conditions[0].Type != "Available" {
		t.Fatalf("%+v", back)
	}
	if ReportWorkloadOf(Workload{Kind: KindCronJob}).Replicas != nil {
		t.Error("a CronJob has no replicas")
	}
}
