package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifierMatchParameterizedRoute(t *testing.T) {
	builder := NewBuilder()
	builder.Handle("SURVEY_GET", http.MethodGet, "/survey/{code}", TierPublicTenant, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	classifier := builder.Classifier()

	req := httptest.NewRequest(http.MethodGet, "/survey/demo", nil)
	if match, ok := classifier.Match(req); !ok {
		t.Fatalf("expected to match parameterized route")
	} else {
		if match.RouteID != "SURVEY_GET" {
			t.Fatalf("unexpected route id %s", match.RouteID)
		}
		if match.Tier != TierPublicTenant {
			t.Fatalf("unexpected tier %s", match.Tier)
		}
	}
}

func TestClassifierAllowedMethodsParameterizedRoute(t *testing.T) {
	builder := NewBuilder()
	builder.Handle("SURVEY_GET", http.MethodGet, "/survey/{code}", TierPublicTenant, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	classifier := builder.Classifier()

	methods, ok := classifier.AllowedMethods("/survey/demo")
	if !ok {
		t.Fatalf("expected allowed methods for parameterized route")
	}

	methodSet := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		methodSet[method] = struct{}{}
	}

	if _, exists := methodSet[http.MethodGet]; !exists {
		t.Fatalf("GET should be allowed for parameterized path")
	}
	if _, exists := methodSet[http.MethodOptions]; !exists {
		t.Fatalf("OPTIONS should be allowed for parameterized path")
	}
}

func TestClassifierMatchParameterizedRouteNotFound(t *testing.T) {
	builder := NewBuilder()
	builder.Handle("SURVEY_GET", http.MethodGet, "/survey/{code}", TierPublicTenant, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	classifier := builder.Classifier()

	req := httptest.NewRequest(http.MethodGet, "/survey", nil)
	if _, ok := classifier.Match(req); ok {
		t.Fatalf("expected parameterized route not to match %s", req.URL.Path)
	}

	if _, ok := classifier.AllowedMethods("/survey"); ok {
		t.Fatalf("expected no allowed methods for %s", "/survey")
	}
}

func TestExtractPathParams(t *testing.T) {
	params, ok := ExtractPathParams("/survey/{code}", "/survey/demo")
	if !ok {
		t.Fatalf("expected params to be extracted")
	}
	if params["code"] != "demo" {
		t.Fatalf("expected code=demo, got %s", params["code"])
	}

	if _, ok := ExtractPathParams("/survey/{code}", "/survey"); ok {
		t.Fatalf("expected mismatch for shorter path")
	}
}
