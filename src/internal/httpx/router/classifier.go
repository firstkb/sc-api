package router

import "net/http"

type Classifier struct {
	routes      []Route
	idx         map[string]Route
	pathMethods map[string]map[string]Route
}

func NewClassifier(routes []Route) *Classifier {
	idx := make(map[string]Route, len(routes))
	pathMethods := make(map[string]map[string]Route)
	for _, rt := range routes {
		idx[rt.Path] = rt
		if pathMethods[rt.URI] == nil {
			pathMethods[rt.URI] = make(map[string]Route)
		}
		pathMethods[rt.URI][rt.Method] = rt
	}
	return &Classifier{routes: routes, idx: idx, pathMethods: pathMethods}
}

type Match struct {
	Tier    Tier
	RouteID RouteID
	Pattern string
}

func (c *Classifier) Match(r *http.Request) (Match, bool) {
	if rt, ok := c.idx[r.Method+" "+r.URL.Path]; ok {
		return Match{Tier: rt.Tier, RouteID: rt.ID, Pattern: rt.Path}, true
	}
	return Match{}, false
}

// AllowedMethods returns list of allowed HTTP methods for a given URI.
func (c *Classifier) AllowedMethods(uri string) ([]string, bool) {
	methodsMap, ok := c.pathMethods[uri]
	if !ok {
		return nil, false
	}
	methods := make([]string, 0, len(methodsMap))
	for method := range methodsMap {
		methods = append(methods, method)
	}
	return methods, true
}
