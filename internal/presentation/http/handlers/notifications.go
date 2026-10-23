package handlers

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	accesstypes "svc-registry/internal/feature/access/types"
	notifytypes "svc-registry/internal/feature/notify/types"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

func (a *Handlers) ListSubscriptions(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	items, err := a.Notify.Subscriptions.ListSubscriptions(c.Context(), p, id)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ItemsOf(items, responses.SubscriptionOf))
}

func (a *Handlers) PutSubscription(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var in requests.Subscription
	if err := bindJSON(c, &in); err != nil {
		return responses.Fail(c, err)
	}
	s, err := a.Notify.Subscriptions.PutSubscription(c.Context(), p, id, c.Params("key"), in.Put())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SubscriptionOf(s))
}

func (a *Handlers) DeleteSubscription(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	if err := a.Notify.Subscriptions.DeleteSubscription(c.Context(), p, id, c.Params("key")); err != nil {
		return responses.Fail(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func (a *Handlers) RotateSigningSecret(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	secret, fingerprint, err := a.Notify.Subscriptions.RotateSigningSecret(c.Context(), p, id, c.Params("key"))
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.SigningSecret{Secret: secret, Fingerprint: fingerprint})
}

func (a *Handlers) TestSubscription(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	r, err := a.Notify.Subscriptions.TestSubscription(c.Context(), p, id, c.Params("key"))
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.TestResultOf(r))
}

func (a *Handlers) ListDeliveries(c fiber.Ctx) error {
	p, _, err := a.currentUser(c)
	if err != nil {
		return responses.Fail(c, err)
	}
	id, err := parseID(c, "id")
	if err != nil {
		return responses.Fail(c, err)
	}
	var q requests.Deliveries
	if err := bindQuery(c, &q, accesstypes.InvalidPagination, map[string]error{
		"subscription": notifytypes.InvalidDeliveryFilter, "status": notifytypes.InvalidDeliveryFilter}); err != nil {
		return responses.Fail(c, err)
	}
	page, err := a.Notify.Subscriptions.ListDeliveries(c.Context(), p, id, q.Filter(), q.Page.Query())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.PageOf(page, responses.DeliveryOf))
}
