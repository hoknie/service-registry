package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type EventType string

const (
	EventDeploymentCreated EventType = "deployment.created"
	EventDeploymentDrift   EventType = "deployment.drift"
	EventLinkCheckFailed   EventType = "link.check_failed"
	EventForgeSyncFailed   EventType = "forge.sync_failed"
	EventDriftResolved     EventType = "deployment.drift_resolved"
	EventLinkRecovered     EventType = "link.check_recovered"
	EventForgeRecovered    EventType = "forge.sync_recovered"
	EventTest              EventType = "test"
)

var SubscribableEvents = []EventType{EventDeploymentCreated, EventDeploymentDrift, EventDriftResolved, EventLinkCheckFailed,
	EventLinkRecovered, EventForgeSyncFailed, EventForgeRecovered}

var RecoveryOf = map[EventType]EventType{EventDeploymentDrift: EventDriftResolved, EventLinkCheckFailed: EventLinkRecovered,
	EventForgeSyncFailed: EventForgeRecovered}

const MaxObserved = 50

type Event struct {
	Type        EventType
	NodeID      uuid.UUID
	Environment *string
	DedupeKey   string
	Episode     string
	OccurredAt  time.Time
	Data        any
}

type Episode struct {
	Key    string
	NodeID uuid.UUID
	Data   json.RawMessage
}

type DeploymentData struct {
	DeploymentID uuid.UUID `json:"deployment_id"`
	Service      string    `json:"service"`
	Environment  string    `json:"environment"`
	Version      string    `json:"version"`
	CommitSHA    *string   `json:"commit_sha"`
	Branch       *string   `json:"branch"`
	URL          *string   `json:"url"`
	DeployedBy   *string   `json:"deployed_by"`
	Source       string    `json:"source"`
	Cluster      *string   `json:"cluster"`
	Namespace    *string   `json:"namespace"`
	Current      bool      `json:"current"`
}

type ObservedVersion struct {
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
}

type DriftData struct {
	Service      string            `json:"service"`
	Environment  string            `json:"environment"`
	Version      string            `json:"version"`
	DeploymentID uuid.UUID         `json:"deployment_id"`
	Observed     []ObservedVersion `json:"observed"`
	Since        string            `json:"since"`
}

type LinkData struct {
	LinkKey        string  `json:"link_key"`
	KindKey        string  `json:"kind_key"`
	Title          *string `json:"title"`
	URL            string  `json:"url"`
	Status         string  `json:"status"`
	HTTPStatus     *int    `json:"http_status"`
	PreviousStatus string  `json:"previous_status"`
	CheckedAt      string  `json:"checked_at"`
}

type ForgeData struct {
	ConnectionID uuid.UUID `json:"connection_id"`
	Forge        string    `json:"forge"`
	OwnerPath    string    `json:"owner_path"`
	RunID        uuid.UUID `json:"run_id"`
	Trigger      string    `json:"trigger"`
	ErrorCode    *string   `json:"error_code"`
	ErrorMessage *string   `json:"error_message"`
}

type DriftResolvedData struct {
	Service      string    `json:"service"`
	Environment  string    `json:"environment"`
	Version      string    `json:"version"`
	DeploymentID uuid.UUID `json:"deployment_id"`
	Since        string    `json:"since"`
}

type TestData struct {
	Subscription string `json:"subscription"`
}

func DeploymentKey(deploymentID uuid.UUID) string { return "dep:" + deploymentID.String() }

func LinkKey(targetID, projectID uuid.UUID, linkKey string, checkID uuid.UUID) string {
	return "link:" + targetID.String() + ":" + projectID.String() + ":" + linkKey + ":" + checkID.String()
}

func ForgeKey(runID uuid.UUID) string { return "forge:" + runID.String() }

func DriftEpisode(projectID uuid.UUID, service, environment string) string {
	sum := sha256.Sum256([]byte(service + "\x00" + environment))
	return "drift:" + projectID.String() + ":" + hex.EncodeToString(sum[:16])
}

func LinkEpisode(targetID, projectID uuid.UUID, linkKey string) string {
	return "link:" + targetID.String() + ":" + projectID.String() + ":" + linkKey
}

func ForgeEpisode(connectionID uuid.UUID) string { return "forge:" + connectionID.String() }

func TestKey(id uuid.UUID) string { return "test:" + id.String() }

func WorkingLink(status string) bool {
	switch status {
	case "ok", "redirect", "auth_required":
		return true
	}
	return false
}

func BrokenLink(status string) bool {
	switch status {
	case "not_found", "server_error", "unexpected", "timeout", "tls_error", "dns_error", "unreachable":
		return true
	}
	return false
}

type NewEvent struct {
	ID         uuid.UUID
	Type       EventType
	NodeID     uuid.UUID
	Payload    []byte
	DedupeKey  string
	OccurredAt time.Time
}

type NewEpisode struct {
	Key       string
	Type      EventType
	NodeID    uuid.UUID
	Signature string
}

func EpisodeEventKey(eventID uuid.UUID) string { return "episode:" + eventID.String() }

func RecoveryEventKey(failureID uuid.UUID) string { return "recovered:" + failureID.String() }
