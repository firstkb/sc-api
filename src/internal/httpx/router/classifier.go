package router

import "net/http"

type Classifier struct {
	routes []Route
	idx    map[string]Route
}

func NewClassifier(routes []Route) *Classifier {
	idx := make(map[string]Route, len(routes))
	for _, rt := range routes {
		idx[rt.Path] = rt
	}
	return &Classifier{routes: routes, idx: idx}
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
