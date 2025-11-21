package router

import "net/http"

type Builder struct {
	mux    *http.ServeMux
	routes []Route
}

func NewBuilder() *Builder { return &Builder{mux: http.NewServeMux()} }

func (b *Builder) Handle(id RouteID, method, path string, tier Tier, h http.Handler) {
	pattern := method + " " + path // "GET /ping"
	b.routes = append(b.routes, Route{ID: id, Method: method, Path: pattern, Tier: tier})
	b.mux.Handle(pattern, h)
}

func (b *Builder) Mux() *http.ServeMux     { return b.mux }
func (b *Builder) Classifier() *Classifier { return NewClassifier(b.routes) }
