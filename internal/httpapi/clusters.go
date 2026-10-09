package httpapi

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"svc-registry/internal/access"
	"svc-registry/internal/httpapi/request"
	"svc-registry/internal/httpapi/response"
	"svc-registry/internal/service"
)

func (a *API) ListClusters(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	items, err := service.ListClusters(c.Context(), a.State, p)
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.ItemsOf(items, response.ClusterOf))
}

func (a *API) CreateCluster(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var in request.Cluster
	if err := bindJSON(c, &in); err != nil {
		return Fail(c, err)
	}
	cl, err := service.CreateCluster(c.Context(), a.State, p, in.Input())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusCreated).JSON(response.ClusterOf(cl))
}

func (a *API) GetCluster(c fiber.Ctx) error {
	return a.clusterCall(c, nil, func(p service.Principal, id uuid.UUID) (any, error) {
		cl, err := service.GetCluster(c.Context(), a.State, p, id)
		return response.ClusterOf(cl), err
	})
}

func (a *API) UpdateCluster(c fiber.Ctx) error {
	var in request.Cluster
	return a.clusterCall(c, &in, func(p service.Principal, id uuid.UUID) (any, error) {
		cl, err := service.UpdateCluster(c.Context(), a.State, p, id, in.Input())
		return response.ClusterOf(cl), err
	})
}

func (a *API) DeleteCluster(c fiber.Ctx) error {
	return a.clusterCall(c, nil, func(p service.Principal, id uuid.UUID) (any, error) {
		return nil, service.DeleteCluster(c.Context(), a.State, p, id)
	})
}

func (a *API) TestCluster(c fiber.Ctx) error {
	return a.clusterCall(c, nil, func(p service.Principal, id uuid.UUID) (any, error) {
		t, err := service.TestCluster(c.Context(), a.State, p, id)
		return response.ClusterTestOf(t), err
	})
}

func (a *API) PollCluster(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	if err := service.PollCluster(c.Context(), a.State, p, id); err != nil {
		return Fail(c, err)
	}
	return c.SendStatus(http.StatusAccepted)
}

func (a *API) UnmatchedWorkloads(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	var q request.Page
	if err := bindQuery(c, &q, access.InvalidPagination, nil); err != nil {
		return Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	page, err := service.UnmatchedWorkloads(c.Context(), a.State, p, id, q.Query())
	if err != nil {
		return Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(response.UnmatchedPage(page))
}

func (a *API) clusterCall(c fiber.Ctx, in any, fn func(service.Principal, uuid.UUID) (any, error)) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return Fail(c, err)
	}
	if in != nil {
		if err := bindJSON(c, in); err != nil {
			return Fail(c, err)
		}
	}
	id, err := parseID(c, "id")
	if err != nil {
		return Fail(c, err)
	}
	body, err := fn(p, id)
	if err != nil {
		return Fail(c, err)
	}
	if body == nil {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(http.StatusOK).JSON(body)
}
