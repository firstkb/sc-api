package tenantsvc

import (
	"testing"
	"time"

	"github.com/firstkb/sc-api/internal/tokencoder"
)

func TestSurveySchemaEncodeDecode(t *testing.T) {
	codec, err := tokencoder.New([]byte("very-secret-key"))
	if err != nil {
		t.Fatalf("codec init: %v", err)
	}

	schema := SurveyTokenSchema{
		Clock: func() time.Time { return time.Unix(1_700_000_000, 0).UTC() },
	}

	token, err := tokencoder.Encode(codec, schema, SurveyTokenPayload{
		TenantID: "tenant-123",
		SurveyID: "survey-456",
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	payload, err := tokencoder.Decode(codec, schema, token)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if payload.TenantID != "tenant-123" || payload.SurveyID != "survey-456" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Nonce == "" {
		t.Fatalf("nonce should be populated")
	}
	if payload.IssuedAt.Unix() != 1_700_000_000 {
		t.Fatalf("unexpected issuedAt %s", payload.IssuedAt)
	}
}

func TestSurveySchemaTamperedToken(t *testing.T) {
	codec, err := tokencoder.New([]byte("very-secret-key"))
	if err != nil {
		t.Fatalf("codec init: %v", err)
	}

	schema := SurveyTokenSchema{}
	token, err := tokencoder.Encode(codec, schema, SurveyTokenPayload{
		TenantID: "tenant-abc",
		SurveyID: "survey-def",
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	tampered := token + "==tamper"
	if _, err := tokencoder.Decode(codec, schema, tampered); err == nil {
		t.Fatalf("expected error for tampered token")
	}
}
