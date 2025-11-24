package router

import (
	"net/http"
	"strings"
)

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
	URI     string
}

func (c *Classifier) Match(r *http.Request) (Match, bool) {
	if rt, ok := c.idx[r.Method+" "+r.URL.Path]; ok {
		return Match{Tier: rt.Tier, RouteID: rt.ID, Pattern: rt.Path, URI: rt.URI}, true
	}

	if rt, ok := c.matchPatternRoute(r.Method, r.URL.Path); ok {
		return Match{Tier: rt.Tier, RouteID: rt.ID, Pattern: rt.Path, URI: rt.URI}, true
	}

	return Match{}, false
}

// AllowedMethods returns list of allowed HTTP methods for a given URI.
func (c *Classifier) AllowedMethods(uri string) ([]string, bool) {
	methodSet := make(map[string]struct{})

	if methodsMap, ok := c.pathMethods[uri]; ok {
		for method := range methodsMap {
			methodSet[method] = struct{}{}
		}
	}

	for _, rt := range c.routes {
		if !isPattern(rt.URI) {
			continue
		}
		if matchPattern(rt.URI, uri) {
			methodSet[rt.Method] = struct{}{}
		}
	}

	if len(methodSet) == 0 {
		return nil, false
	}

	methods := make([]string, 0, len(methodSet))
	for method := range methodSet {
		methods = append(methods, method)
	}

	return methods, true
}

func (c *Classifier) matchPatternRoute(method, path string) (Route, bool) {
	for _, rt := range c.routes {
		if rt.Method != method || !isPattern(rt.URI) {
			continue
		}
		if matchPattern(rt.URI, path) {
			return rt, true
		}
	}
	return Route{}, false
}

func matchPattern(pattern, path string) bool {
	_, ok := ExtractPathParams(pattern, path)
	return ok
}

func splitPath(p string) []string {
	if p == "" || p == "/" {
		return nil
	}

	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return nil
	}

	return strings.Split(trimmed, "/")
}

func isPattern(path string) bool {
	return strings.Contains(path, "{") && strings.Contains(path, "}")
}

func isParamSegment(segment string) bool {
	return len(segment) >= 2 && segment[0] == '{' && segment[len(segment)-1] == '}'
}

// ExtractPathParams возвращает значения переменных пути согласно шаблону.
// Пример: pattern /survey/{code} и path /survey/demo -> map["code"]="demo".
func ExtractPathParams(pattern, path string) (map[string]string, bool) {
	patternSegments := splitPath(pattern)
	pathSegments := splitPath(path)

	if len(patternSegments) != len(pathSegments) {
		return nil, false
	}

	var params map[string]string
	for i := range patternSegments {
		segment := patternSegments[i]
		if isParamSegment(segment) {
			if params == nil {
				params = make(map[string]string)
			}
			params[segment[1:len(segment)-1]] = pathSegments[i]
			continue
		}
		if segment != pathSegments[i] {
			return nil, false
		}
	}

	return params, true
}
