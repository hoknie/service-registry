package services

import (
	catalogservices "svc-registry/internal/feature/catalog/services"
	notifyports "svc-registry/internal/feature/notify/ports"
	"svc-registry/internal/feature/notify/repositories"
	ownershipservices "svc-registry/internal/feature/ownership/services"
	"svc-registry/internal/platform/config"
	"svc-registry/pkg/pgtx"
	"svc-registry/pkg/secretbox"
)

type Deps struct {
	DB        *pgtx.DB
	Config    *config.Config
	Catalog   *catalogservices.CatalogServices
	Ownership *ownershipservices.OwnershipServices
	Sender    notifyports.SenderPort
	Secrets   *secretbox.Box
	Version   string
}

type parts struct {
	cfg                    *config.Config
	db                     *pgtx.DB
	catalog                *catalogservices.CatalogServices
	ownership              *ownershipservices.OwnershipServices
	sender                 notifyports.SenderPort
	secrets                *secretbox.Box
	version                string
	subscriptionRepository *repositories.SubscriptionRepository
	outboxRepository       *repositories.OutboxRepository
	episodeRepository      *repositories.EpisodeRepository
}

func newParts(d Deps) *parts {
	return &parts{
		cfg:                    d.Config,
		db:                     d.DB,
		catalog:                d.Catalog,
		ownership:              d.Ownership,
		sender:                 d.Sender,
		secrets:                d.Secrets,
		version:                d.Version,
		subscriptionRepository: repositories.NewSubscriptionRepository(d.DB),
		outboxRepository:       repositories.NewOutboxRepository(d.DB),
		episodeRepository:      repositories.NewEpisodeRepository(d.DB),
	}
}

type NotifyServices struct {
	Outbox        *OutboxService
	Deliveries    *DeliveriesService
	Subscriptions *SubscriptionsService
	Ports         *PortsService
}

func New(d Deps) *NotifyServices {
	p := newParts(d)
	outbox := &OutboxService{catalog: p.catalog, cfg: p.cfg, db: p.db, episodeRepository: p.episodeRepository,
		ownership: p.ownership, outboxRepository: p.outboxRepository, subscriptionRepository: p.subscriptionRepository}
	deliveries := &DeliveriesService{catalog: p.catalog, cfg: p.cfg, outboxRepository: p.outboxRepository, secrets: p.secrets,
		sender: p.sender, subscriptionRepository: p.subscriptionRepository, version: p.version}
	return &NotifyServices{
		Outbox:     outbox,
		Deliveries: deliveries,
		Subscriptions: &SubscriptionsService{catalog: p.catalog, db: p.db, deliveries: deliveries, outbox: outbox,
			outboxRepository: p.outboxRepository, secrets: p.secrets, subscriptionRepository: p.subscriptionRepository},
		Ports: &PortsService{secrets: p.secrets, subscriptionRepository: p.subscriptionRepository},
	}
}
