package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"svc-registry/internal/feature/publishing/repositories"
	"svc-registry/internal/platform/apperr"
)

const hostsTTL = 15 * time.Second

type HostsService struct {
	publicationRepository *repositories.PublicationRepository
	mu                    sync.Mutex
	hosts                 map[string]uuid.UUID
	loadedAt              time.Time
}

func (s *HostsService) PublicationHost(ctx context.Context, host string) (uuid.UUID, bool, error) {
	host = strings.ToLower(host)
	if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hosts == nil || time.Since(s.loadedAt) > hostsTTL {
		hosts, err := s.publicationRepository.Hosts(ctx)
		if err != nil {
			return uuid.Nil, false, apperr.Wrap(err)
		}
		s.hosts, s.loadedAt = hosts, time.Now()
	}
	id, ok := s.hosts[host]
	return id, ok, nil
}

func (s *HostsService) forget() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hosts = nil
}
