package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/feature/access"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListClusters(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Deploy.ListClusters(c.Context(), p)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.ClusterOf))
}

func (a *Handlers) CreateCluster(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Cluster
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	cl, err := a.Deploy.CreateCluster(c.Context(), p, in.Input())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(responses.ClusterOf(cl))
}

func (a *Handlers) GetCluster(c fiber.Ctx) error {
	return a.clusterCall(c, nil, func(p access.Principal, id uuid.UUID) (any, error) {
		cl, err := a.Deploy.GetCluster(c.Context(), p, id)
		return responses.ClusterOf(cl), err
	})
}

func (a *Handlers) UpdateCluster(c fiber.Ctx) error {
	var in requests.Cluster
	return a.clusterCall(c, &in, func(p access.Principal, id uuid.UUID) (any, error) {
		cl, err := a.Deploy.UpdateCluster(c.Context(), p, id, in.Input())
		return responses.ClusterOf(cl), err
	})
}

func (a *Handlers) DeleteCluster(c fiber.Ctx) error {
	return a.clusterCall(c, nil, func(p access.Principal, id uuid.UUID) (any, error) {
		return nil, a.Deploy.DeleteCluster(c.Context(), p, id)
	})
}

func (a *Handlers) TestCluster(c fiber.Ctx) error {
	return a.clusterCall(c, nil, func(p access.Principal, id uuid.UUID) (any, error) {
		t, err := a.Deploy.TestCluster(c.Context(), p, id)
		return responses.ClusterTestOf(t), err
	})
}

func (a *Handlers) PollCluster(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Deploy.PollCluster(c.Context(), p, id); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusAccepted)
}

func (a *Handlers) UnmatchedWorkloads(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Page
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Deploy.UnmatchedWorkloads(c.Context(), p, id, q.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.UnmatchedPage(page))
}

func (a *Handlers) clusterCall(c fiber.Ctx, in any, fn func(access.Principal, uuid.UUID) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	if in != nil {
		if err := bindJSON(c, in); err != nil {
			return responses.Fail(c, err)
		}
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	body, err := fn(p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	if body == nil {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(http.StatusOK).JSON(body)
}
