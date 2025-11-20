package pingsvc

import (
	"context"
	"testing"
)

func TestPingService_GetPing(t *testing.T) {
	svc := &PingService{}
	svc.repository = &Repo{}

	resp, err := svc.GetPing(context.Background())
	if err != nil {
		t.Fatalf("GetPing returned error: %v", err)
	}
	if resp == nil || resp.Message != "pong" {
		t.Fatalf("unexpected ping response: %+v", resp)
	}
}
