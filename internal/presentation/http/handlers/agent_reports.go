package handlers

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	deployservices "svc-registry/internal/feature/deploy/services"
	deploytypes "svc-registry/internal/feature/deploy/types"
	"svc-registry/internal/platform/apperr"
	"svc-registry/internal/presentation/http/requests"
	"svc-registry/internal/presentation/http/responses"
)

var (
	errReportTooLarge = errors.New("the report is too large")
	errReportGzip     = errors.New("the body is not valid gzip")
)

func reportTooLarge(limit int64) *responses.APIError {
	return responses.NewAPIError(http.StatusRequestEntityTooLarge, "ingest.payload_too_large",
		"the report must be at most "+strconv.Itoa(deploytypes.ReportWireLimit)+" bytes as sent and "+
			strconv.FormatInt(limit, 10)+" bytes after decompression")
}

func (a *Handlers) authenticateAgent(c fiber.Ctx, rawClusterID string) (deployservices.Agent, error) {
	id, err := uuid.Parse(rawClusterID)
	if err != nil {
		return deployservices.Agent{}, apperr.New(apperr.InvalidAgentToken)
	}
	var token *string
	if t, err := bearerKey.Extract(c); err == nil {
		token = &t
	}
	return a.Deploy.Reports.AuthenticateAgent(c.Context(), id, token)
}

func (a *Handlers) AgentConfig(c fiber.Ctx) error {
	who, err := a.authenticateAgent(c, c.Params("id"))
	if err != nil {
		return responses.Fail(c, err)
	}
	cfg, err := a.Deploy.Reports.AgentConfig(c.Context(), who)
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.AgentConfigOf(cfg))
}

func (a *Handlers) AgentReport(c fiber.Ctx) error {
	who, err := a.authenticateAgent(c, c.Params("id"))
	if err != nil {
		return responses.Fail(c, err)
	}
	if mt, _, err := mime.ParseMediaType(c.Get(fiber.HeaderContentType)); err != nil || mt != fiber.MIMEApplicationJSON {
		return InvalidBody("Content-Type must be application/json").Send(c)
	}
	in, failure := a.decodeReport(c)
	if failure != nil {
		return failure.Send(c)
	}
	answer, err := a.Deploy.Reports.AcceptReport(c.Context(), who, in.Domain())
	if err != nil {
		return responses.Fail(c, err)
	}
	return c.Status(http.StatusOK).JSON(responses.ReportAnswerOf(answer))
}

func (a *Handlers) decodeReport(c fiber.Ctx) (requests.AgentReport, *responses.APIError) {
	var in requests.AgentReport
	limit := a.Config.K8s.AgentReportMaxBytes
	var body io.Reader = bytes.NewReader(c.BodyRaw())
	gzipped := false
	switch enc := strings.ToLower(strings.TrimSpace(c.Get(fiber.HeaderContentEncoding))); enc {
	case "", "identity":
	case "gzip":
		zr, err := gzip.NewReader(body)
		if err != nil {
			return in, InvalidBody(errReportGzip.Error())
		}
		body, gzipped = zr, true
	default:
		return in, InvalidBody("Content-Encoding must be gzip or absent")
	}
	br := bufio.NewReader(&cappedReader{r: body, left: limit, gzipped: gzipped})
	err := decodeObject(br, &in)
	switch {
	case err == nil:
		return in, nil
	case errors.Is(err, errReportTooLarge):
		return in, reportTooLarge(limit)
	case errors.Is(err, errReportGzip):
		return in, InvalidBody(errReportGzip.Error())
	}
	return in, InvalidBody(err.Error())
}

func decodeObject(br *bufio.Reader, out any) error {
	for {
		b, err := br.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errors.New("the body must be a JSON object")
			}
			return err
		}
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			continue
		}
		if b != '{' {
			return errors.New("the body must be a JSON object")
		}
		_ = br.UnreadByte()
		break
	}
	dec := json.NewDecoder(br)
	if err := dec.Decode(out); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("the body must hold one JSON object")
		}
		return err
	}
	return nil
}

type cappedReader struct {
	r       io.Reader
	left    int64
	gzipped bool
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.left <= 0 {
		var one [1]byte
		n, err := c.r.Read(one[:])
		if n > 0 {
			return 0, errReportTooLarge
		}
		return 0, c.mark(err)
	}
	if int64(len(p)) > c.left {
		p = p[:c.left]
	}
	n, err := c.r.Read(p)
	c.left -= int64(n)
	return n, c.mark(err)
}

func (c *cappedReader) mark(err error) error {
	if err == nil || errors.Is(err, io.EOF) || !c.gzipped {
		return err
	}
	return fmt.Errorf("%w: %w", errReportGzip, err)
}

func (a *Handlers) AgentReportTooLarge(c fiber.Ctx, clusterSegment string) error {
	if _, err := a.authenticateAgent(c, clusterSegment); err != nil {
		return responses.Fail(c, err)
	}
	return reportTooLarge(a.Config.K8s.AgentReportMaxBytes).Send(c)
}
