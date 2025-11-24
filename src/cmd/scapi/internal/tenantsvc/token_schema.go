package tenantsvc

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/firstkb/sc-api/internal/tokencoder"
)

// SurveyTokenPayload описывает содержимое survey-кода.
type SurveyTokenPayload struct {
	TenantID string
	SurveyID string
	IssuedAt time.Time
	Nonce    string
}

// SurveyTokenSchema реализует Schema с разделителем "|".
type SurveyTokenSchema struct {
	Separator string
	NonceSize int
	Clock     func() time.Time
}

func (s SurveyTokenSchema) sep() string {
	if s.Separator == "" {
		return "|"
	}
	return s.Separator
}

func (s SurveyTokenSchema) nonceSize() int {
	if s.NonceSize <= 0 {
		return 16
	}
	return s.NonceSize
}

func (s SurveyTokenSchema) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now().UTC()
}

func (s SurveyTokenSchema) Marshal(payload SurveyTokenPayload) ([]byte, error) {
	if payload.TenantID == "" || payload.SurveyID == "" {
		return nil, errors.New("survey schema: tenantID and surveyID required")
	}
	if payload.IssuedAt.IsZero() {
		payload.IssuedAt = s.now()
	}
	if payload.Nonce == "" {
		nonce, err := tokencoder.GenerateNonce(s.nonceSize())
		if err != nil {
			return nil, err
		}
		payload.Nonce = nonce
	}

	builder := []string{
		payload.TenantID,
		payload.SurveyID,
		strconv.FormatInt(payload.IssuedAt.Unix(), 10),
		payload.Nonce,
	}
	return []byte(strings.Join(builder, s.sep())), nil
}

func (s SurveyTokenSchema) Unmarshal(data []byte) (SurveyTokenPayload, error) {
	var payload SurveyTokenPayload
	parts := strings.Split(string(data), s.sep())
	if len(parts) != 4 {
		return payload, errors.New("survey schema: invalid payload")
	}
	ts, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return payload, err
	}

	payload.TenantID = parts[0]
	payload.SurveyID = parts[1]
	payload.IssuedAt = time.Unix(ts, 0).UTC()
	payload.Nonce = parts[3]
	return payload, nil
}
