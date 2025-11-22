package router

import "net/http"

type Builder struct {
	mux      *http.ServeMux
	routes   []Route
	patterns map[string]struct{}
}

func NewBuilder() *Builder {
	return &Builder{
		mux:      http.NewServeMux(),
		patterns: make(map[string]struct{}),
	}
}

func (b *Builder) Handle(id RouteID, method, path string, tier Tier, h http.Handler) {
	pattern := method + " " + path // "GET /ping"
	if _, exists := b.patterns[pattern]; !exists {
		b.routes = append(b.routes, Route{ID: id, Method: method, Path: pattern, URI: path, Tier: tier})
		b.patterns[pattern] = struct{}{}
		b.mux.Handle(pattern, h)
	}

	// Automatically add OPTIONS-preflight if it hasn't been registered yet.
	if method != http.MethodOptions {
		optPattern := http.MethodOptions + " " + path
		if _, exists := b.patterns[optPattern]; !exists {
			b.routes = append(b.routes, Route{ID: id, Method: http.MethodOptions, Path: optPattern, URI: path, Tier: tier})
			b.patterns[optPattern] = struct{}{}
		}
	}
}

func (b *Builder) Mux() *http.ServeMux     { return b.mux }
func (b *Builder) Classifier() *Classifier { return NewClassifier(b.routes) }
