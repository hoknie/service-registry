package services

import (
	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"

	apistypes "svc-registry/internal/feature/apis/types"
	knowledgeservices "svc-registry/internal/feature/knowledge/services"
)

const cacheSize = 256

type Deps struct {
	Knowledge *knowledgeservices.KnowledgeServices
}

type cacheKey struct {
	snapshot uuid.UUID
	path     string
}

type ApisServices struct {
	Specs *SpecsService
}

func New(d Deps) *ApisServices {
	cache, _ := lru.New[cacheKey, apistypes.Document](cacheSize)
	return &ApisServices{Specs: &SpecsService{documents: d.Knowledge.Documents, cache: cache}}
}
