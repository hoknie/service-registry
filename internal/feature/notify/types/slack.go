package types

import (
	"encoding/json"
	"strconv"
	"strings"
)

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackBlock struct {
	Type     string      `json:"type"`
	Text     *slackText  `json:"text,omitempty"`
	Fields   []slackText `json:"fields,omitempty"`
	Elements []any       `json:"elements,omitempty"`
}

type slackButton struct {
	Type string    `json:"type"`
	Text slackText `json:"text"`
	URL  string    `json:"url"`
}

type slackMessage struct {
	Text   string       `json:"text"`
	Blocks []slackBlock `json:"blocks"`
}

var slackEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func escape(s string) string { return slackEscaper.Replace(s) }

type field struct{ label, value string }

func SlackBody(p Payload, locale string) ([]byte, error) {
	m := MessagesFor(locale)
	headline, fields := summary(p, m)
	text := m[string(p.Type)] + " · " + escape(p.Node.Path)
	if headline != "" {
		text += " · " + escape(headline)
	}
	blocks := []slackBlock{{Type: "section", Text: &slackText{Type: "mrkdwn", Text: "*" + text + "*"}}}
	if len(fields) > 0 {
		b := slackBlock{Type: "section"}
		for _, f := range fields {
			b.Fields = append(b.Fields, slackText{Type: "mrkdwn", Text: "*" + m[f.label] + "*\n" + escape(f.value)})
		}
		blocks = append(blocks, b)
	}
	if owners := ownersLine(p.Owners, m); owners != "" {
		blocks = append(blocks, slackBlock{Type: "context", Elements: []any{slackText{Type: "mrkdwn", Text: "*" + m["owners"] + ":* " + owners}}})
	}
	if p.Node.URL != nil {
		blocks = append(blocks, slackBlock{Type: "actions", Elements: []any{slackButton{Type: "button",
			Text: slackText{Type: "plain_text", Text: m["open"]}, URL: *p.Node.URL}}})
	}
	return json.Marshal(slackMessage{Text: text, Blocks: blocks})
}

func ownersLine(owners []Owner, m Messages) string {
	parts := make([]string, 0, len(owners))
	for _, o := range owners {
		part := escape(o.Name)
		if o.OnCallURL != nil {
			part += " (<" + escape(*o.OnCallURL) + "|" + m["on_call"] + ">)"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

func summary(p Payload, m Messages) (string, []field) {
	switch p.Type {
	case EventDeploymentCreated:
		var d DeploymentData
		if json.Unmarshal(p.Data, &d) != nil {
			return "", nil
		}
		fields := []field{{"service", d.Service}, {"environment", d.Environment}, {"version", d.Version}, {"source", d.Source}}
		fields = appendOpt(fields, "commit", d.CommitSHA)
		fields = appendOpt(fields, "branch", d.Branch)
		fields = appendOpt(fields, "cluster", d.Cluster)
		return d.Version + " → " + d.Environment, fields
	case EventDeploymentDrift:
		var d DriftData
		if json.Unmarshal(p.Data, &d) != nil {
			return "", nil
		}
		observed := make([]string, 0, len(d.Observed))
		for _, o := range d.Observed {
			observed = append(observed, o.Version+" ("+o.Cluster+"/"+o.Namespace+")")
		}
		return d.Version + " ≠ " + strings.Join(observed, ", "), []field{{"service", d.Service}, {"environment", d.Environment},
			{"version", d.Version}, {"observed", strings.Join(observed, ", ")}, {"since", d.Since}}
	case EventDriftResolved:
		var d DriftResolvedData
		if json.Unmarshal(p.Data, &d) != nil {
			return "", nil
		}
		return d.Version + " → " + d.Environment, []field{{"service", d.Service}, {"environment", d.Environment},
			{"version", d.Version}, {"since", d.Since}}
	case EventLinkCheckFailed, EventLinkRecovered:
		var d LinkData
		if json.Unmarshal(p.Data, &d) != nil {
			return "", nil
		}
		status := d.Status
		if d.HTTPStatus != nil {
			status += " (HTTP " + strconv.Itoa(*d.HTTPStatus) + ")"
		}
		return d.LinkKey + ": " + status, []field{{"link", d.URL}, {"status", status}, {"previous", d.PreviousStatus}}
	case EventRepoSyncFailed, EventRepoSyncRecovered:
		var d RepoSyncData
		if json.Unmarshal(p.Data, &d) != nil {
			return "", nil
		}
		if p.Type == EventRepoSyncRecovered {
			return d.Hosting + " " + d.OwnerPath, []field{{"connection", d.Hosting + " " + d.OwnerPath}}
		}
		fields := []field{{"connection", d.Hosting + " " + d.OwnerPath}}
		fields = appendOpt(fields, "error", d.ErrorMessage)
		code := ""
		if d.ErrorCode != nil {
			code = *d.ErrorCode
		}
		return code, fields
	case EventTest:
		var d TestData
		if json.Unmarshal(p.Data, &d) != nil {
			return "", nil
		}
		return d.Subscription, []field{{"subscription", d.Subscription}}
	}
	return "", nil
}

func appendOpt(fields []field, label string, v *string) []field {
	if v == nil || *v == "" {
		return fields
	}
	return append(fields, field{label, *v})
}
