package types

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	SigningPrefix   = "whsec_"
	signingKeyBytes = 32
)

var errSigningKey = errors.New("signing key is not whsec_<base64>")

func NewSigningSecret() (string, error) {
	key := make([]byte, signingKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return SigningPrefix + base64.StdEncoding.EncodeToString(key), nil
}

func Sign(secret, id string, ts int64, body []byte) (string, error) {
	raw, ok := strings.CutPrefix(secret, SigningPrefix)
	if !ok {
		return "", errSigningKey
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", errSigningKey
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + strconv.FormatInt(ts, 10) + "."))
	mac.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

func WebhookRequest(url string, deliveryID uuid.UUID, body []byte, signing *string, at time.Time, userAgent string) (Request, error) {
	ts := at.Unix()
	headers := map[string]string{
		"Content-Type":      "application/json",
		"User-Agent":        userAgent,
		"webhook-id":        deliveryID.String(),
		"webhook-timestamp": strconv.FormatInt(ts, 10),
	}
	if signing != nil {
		sig, err := Sign(*signing, deliveryID.String(), ts, body)
		if err != nil {
			return Request{}, err
		}
		headers["webhook-signature"] = sig
	}
	return Request{URL: url, Body: body, Headers: headers}, nil
}
