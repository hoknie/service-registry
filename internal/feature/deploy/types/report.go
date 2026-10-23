package types

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Mode string

const (
	ModePoll  Mode = "poll"
	ModeAgent Mode = "agent"
)

func ParseMode(raw string) (Mode, error) {
	switch m := Mode(raw); m {
	case ModePoll, ModeAgent:
		return m, nil
	}
	return "", InvalidClusterMode
}

const (
	ReportWireLimit           = 16 << 20
	MaxReportWorkloads        = 100000
	DefaultWorkloadsPerGapSec = 2000
	MaxReportMissing          = 20
	MaxAgentVersionLen        = 64
	MaxReportMessage          = 1024
)

type AgentState struct {
	Version         *string
	LastReportAt    *string
	TokenPrefixes   []string
	Report          *ReportSummary
	ReportGapSecs   int32
	ReportWorkloads int32
}

type ReportSummary struct {
	KubernetesVersion *string      `json:"kubernetes_version"`
	Missing           []string     `json:"missing"`
	Error             *ReportError `json:"error"`
}

type ReportError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ReportContainer struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

type ReportReplicas struct {
	Desired int32 `json:"desired"`
	Ready   int32 `json:"ready"`
	Updated int32 `json:"updated"`
}

type ReportCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type ReportPodContainer struct {
	Name     string `json:"name"`
	ImageID  string `json:"image_id"`
	Restarts int32  `json:"restarts"`
}

type ReportPod struct {
	Containers []ReportPodContainer `json:"containers"`
}

type ReportJob struct {
	StartedAt *time.Time `json:"started_at"`
	Active    int32      `json:"active"`
	Succeeded int32      `json:"succeeded"`
	Failed    int32      `json:"failed"`
}

type ReportWorkload struct {
	UID                 string            `json:"uid"`
	Kind                Kind              `json:"kind"`
	Namespace           string            `json:"namespace"`
	Name                string            `json:"name"`
	Labels              map[string]string `json:"labels"`
	Annotations         map[string]string `json:"annotations"`
	TemplateLabels      map[string]string `json:"template_labels"`
	TemplateAnnotations map[string]string `json:"template_annotations"`
	Selector            string            `json:"selector"`
	Containers          []ReportContainer `json:"containers"`
	Generation          int64             `json:"generation"`
	ObservedGeneration  int64             `json:"observed_generation"`
	Replicas            *ReportReplicas   `json:"replicas"`
	Conditions          []ReportCondition `json:"conditions"`
	Schedule            string            `json:"schedule"`
	Suspended           bool              `json:"suspended"`
	Pods                *[]ReportPod      `json:"pods"`
	Jobs                *[]ReportJob      `json:"jobs"`
}

type Report struct {
	AgentVersion      string           `json:"agent_version"`
	KubernetesVersion *string          `json:"kubernetes_version"`
	Missing           []string         `json:"missing"`
	Error             *ReportError     `json:"error"`
	Workloads         []ReportWorkload `json:"workloads"`
}

type AgentCluster struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type AgentConfig struct {
	Cluster       AgentCluster `json:"cluster"`
	Enabled       bool         `json:"enabled"`
	Namespaces    []string     `json:"namespaces"`
	Rules         []Rule       `json:"rules"`
	IntervalSecs  int32        `json:"interval_secs"`
	ReportGapSecs int32        `json:"report_gap_secs"`
}

type ReportCounts struct {
	Workloads int `json:"workloads"`
	Matched   int `json:"matched"`
	Unmatched int `json:"unmatched"`
}

type RegistryError struct {
	Status     int
	Code       string
	RetryAfter time.Duration
}

func (e *RegistryError) Error() string {
	return "the registry answered " + strconv.Itoa(e.Status) + " " + e.Code
}

type ReportLease struct {
	Leased, Disabled, Busy bool
	RetryAfter             int64
}

type ReportAnswer struct {
	AgentConfig
	Accepted ReportCounts `json:"accepted"`
}

func ConfigOf(c Cluster, gapSecs int32) AgentConfig {
	return AgentConfig{Cluster: AgentCluster{ID: c.ID, Name: c.Name}, Enabled: c.Enabled, Namespaces: c.Namespaces,
		Rules: c.Rules, IntervalSecs: c.IntervalSecs, ReportGapSecs: gapSecs}
}

func ValidateReport(r Report) error {
	if n := utf8.RuneCountInString(r.AgentVersion); n < 1 || n > MaxAgentVersionLen ||
		(r.KubernetesVersion != nil && utf8.RuneCountInString(*r.KubernetesVersion) > MaxAgentVersionLen) ||
		len(r.Missing) > MaxReportMissing {
		return InvalidReport
	}
	for _, m := range r.Missing {
		if m == "" || len(m) > 255 {
			return InvalidReport
		}
	}
	if r.Error != nil {
		if !ReportedFailure(r.Error.Code) || utf8.RuneCountInString(r.Error.Message) > MaxReportMessage {
			return InvalidReport
		}
		return nil
	}
	if len(r.Workloads) > MaxReportWorkloads {
		return InvalidReport
	}
	for _, w := range r.Workloads {
		switch w.Kind {
		case KindDeployment, KindStatefulSet, KindDaemonSet, KindCronJob:
		default:
			return InvalidReport
		}
		if w.UID == "" || w.Namespace == "" || w.Name == "" {
			return InvalidReport
		}
	}
	return nil
}

func (w ReportWorkload) Workload() Workload {
	out := Workload{UID: w.UID, Kind: w.Kind, Namespace: w.Namespace, Name: w.Name, Labels: w.Labels,
		Annotations: w.Annotations, TemplateLabels: w.TemplateLabels, TemplateAnnotations: w.TemplateAnnotations,
		Selector: w.Selector, Generation: w.Generation, ObservedGeneration: w.ObservedGeneration,
		Schedule: w.Schedule, Suspended: w.Suspended}
	for _, c := range w.Containers {
		out.Containers = append(out.Containers, Container(c))
	}
	for _, c := range w.Conditions {
		out.Conditions = append(out.Conditions, Condition(c))
	}
	if w.Replicas != nil {
		out.Desired, out.Ready, out.Updated = w.Replicas.Desired, w.Replicas.Ready, w.Replicas.Updated
	}
	return out
}

func ReportWorkloadOf(w Workload) ReportWorkload {
	out := ReportWorkload{UID: w.UID, Kind: w.Kind, Namespace: w.Namespace, Name: w.Name, Labels: w.Labels,
		Annotations: w.Annotations, TemplateLabels: w.TemplateLabels, TemplateAnnotations: w.TemplateAnnotations,
		Selector: w.Selector, Generation: w.Generation, ObservedGeneration: w.ObservedGeneration,
		Schedule: w.Schedule, Suspended: w.Suspended, Containers: []ReportContainer{}, Conditions: []ReportCondition{}}
	for _, c := range w.Containers {
		out.Containers = append(out.Containers, ReportContainer(c))
	}
	for _, c := range w.Conditions {
		out.Conditions = append(out.Conditions, ReportCondition(c))
	}
	if w.Kind != KindCronJob {
		out.Replicas = &ReportReplicas{Desired: w.Desired, Ready: w.Ready, Updated: w.Updated}
	}
	return out
}

func PodsOf(pods []ReportPod) []Pod {
	out := make([]Pod, 0, len(pods))
	for _, p := range pods {
		pod := Pod{}
		for _, c := range p.Containers {
			pod.Containers = append(pod.Containers, PodContainer(c))
		}
		out = append(out, pod)
	}
	return out
}

func ReportPodsOf(pods []Pod) []ReportPod {
	out := make([]ReportPod, 0, len(pods))
	for _, p := range pods {
		pod := ReportPod{Containers: []ReportPodContainer{}}
		for _, c := range p.Containers {
			pod.Containers = append(pod.Containers, ReportPodContainer(c))
		}
		out = append(out, pod)
	}
	return out
}

func JobsOf(jobs []ReportJob) []Job {
	out := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, Job(j))
	}
	return out
}

func ReportJobsOf(jobs []Job) []ReportJob {
	out := make([]ReportJob, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, ReportJob(j))
	}
	return out
}

func Candidate(w Workload, rules []Rule) bool {
	if _, ok := w.Annotations[AnnotationProject]; ok {
		return true
	}
	if _, ok := w.TemplateAnnotations[AnnotationProject]; ok {
		return true
	}
	for _, r := range rules {
		if _, ok := w.Labels[r.Label]; ok {
			return true
		}
	}
	return false
}

func SummaryOf(r Report) ReportSummary {
	missing := r.Missing
	if missing == nil {
		missing = []string{}
	}
	return ReportSummary{KubernetesVersion: r.KubernetesVersion, Missing: missing, Error: r.Error}
}

func (e *ReportError) Failure() *Failure {
	if e == nil {
		return nil
	}
	return &Failure{Code: FailureCode(e.Code), Message: strings.TrimSpace(e.Message)}
}
