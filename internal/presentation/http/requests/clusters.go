package requests

import (
	"svc-registry/internal/feature/deploy"
	"svc-registry/internal/feature/forge"
)

type ClusterCredentials struct {
	Token    *string `json:"token"`
	TokenRef *string `json:"token_ref"`
}

type Cluster struct {
	Name         *string             `json:"name"`
	Environment  *string             `json:"environment"`
	InCluster    *bool               `json:"in_cluster"`
	APIURL       *string             `json:"api_url"`
	CAPEM        *string             `json:"ca_pem"`
	Credentials  *ClusterCredentials `json:"credentials"`
	Namespaces   *[]string           `json:"namespaces"`
	Rules        *[]deploy.Rule      `json:"rules"`
	IntervalSecs *int64              `json:"interval_secs"`
	Enabled      *bool               `json:"enabled"`
}

func (r Cluster) Input() deploy.ClusterInput {
	in := deploy.ClusterInput{Name: r.Name, Environment: r.Environment, InCluster: r.InCluster, APIURL: r.APIURL,
		CAPEM: r.CAPEM, Namespaces: r.Namespaces, Rules: r.Rules, IntervalSecs: r.IntervalSecs, Enabled: r.Enabled}
	if r.Credentials != nil {
		in.Credentials = &forge.CredentialsInput{Token: r.Credentials.Token, TokenRef: r.Credentials.TokenRef}
	}
	return in
}
