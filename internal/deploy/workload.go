package deploy

import "time"

type Kind string

const (
	KindDeployment  Kind = "Deployment"
	KindStatefulSet Kind = "StatefulSet"
	KindDaemonSet   Kind = "DaemonSet"
	KindCronJob     Kind = "CronJob"
)

type Container struct {
	Name  string
	Image string
}

type Condition struct {
	Type   string
	Status string
	Reason string
}

type Workload struct {
	UID                 string
	Kind                Kind
	Namespace           string
	Name                string
	Labels              map[string]string
	Annotations         map[string]string
	TemplateLabels      map[string]string
	TemplateAnnotations map[string]string
	Selector            string
	Containers          []Container
	Generation          int64
	ObservedGeneration  int64
	Desired             int32
	Ready               int32
	Updated             int32
	Conditions          []Condition
	Schedule            string
	Suspended           bool
}

type PodContainer struct {
	Name     string
	ImageID  string
	Restarts int32
}

type Pod struct {
	Containers []PodContainer
}

type Job struct {
	StartedAt *time.Time
	Active    int32
	Succeeded int32
	Failed    int32
}
