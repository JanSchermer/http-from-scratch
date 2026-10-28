package http1

import (
	"fmt"
	"net/http"
	"strings"
)

type ServeMux struct {
	pathRootSegments []*muxPathSegment
	notFoundHandler  Handler
}

type muxPathSegment struct {
	method          string
	host            string
	pattern         string
	handler         *Handler
	segmentName     string // Is wildcard if empty
	matchRemainder  bool
	subPathSegments []*muxPathSegment
}

// returns (method, host, []segmentString, error)
func parsePattern(pattern string) (string, string, []string, error) {
	var method string
	switch strings.Count(pattern, " ") {
	case 0:
		method = ""
	case 1:
		splits := strings.Split(pattern, " ")
		method = strings.ToUpper(splits[0])
		pattern = splits[1]
	default:
		return "", "", nil, fmt.Errorf("invalid pattern: %s", pattern)
	}
	if !strings.Contains(pattern, "/") {
		return "", "", nil, fmt.Errorf("invalid pattern: %s", pattern)
	}
	splits := strings.Split(pattern, "/")
	for i := range splits {
		splits[i] = strings.ToLower(splits[i])
	}
	host := splits[0]
	// Check only valid characters in splits
	return method, host, splits[1:], nil
}

func isSegmentWildcard(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func segmentOrEmptyIfWildcard(segment string) string {
	if isSegmentWildcard(segment) {
		return ""
	}
	return segment
}

func findPathSegment(rootSegments []*muxPathSegment, method string, host string, segmentStrings []string) (*muxPathSegment, []string) {
	if len(segmentStrings) == 0 {
		return nil, segmentStrings
	}
	var match *muxPathSegment
	for _, segment := range rootSegments {
		if (segment.host != host && segment.host != "") || (host == "" && segment.host != "") {
			continue
		}
		if segment.method != method && segment.method != "" {
			continue
		}
		if (segment.segmentName != segmentStrings[0] && segment.segmentName != "" && !isSegmentWildcard(segmentStrings[0])) ||
			((segmentStrings[0] == "" || isSegmentWildcard(segmentStrings[0])) && segment.segmentName != "") {
			continue
		}
		if !segment.matchRemainder && len(segmentStrings) > 1 {
			continue
		}
		if match == nil {
			match = segment
		} else if match.host == "" && segment.host != "" {
			match = segment
		} else if match.method == "" && segment.method != "" {
			match = segment
		} else if match.segmentName == "" && segment.segmentName != "" && !isSegmentWildcard(segmentStrings[0]) {
			match = segment
		} else if match.matchRemainder && !segment.matchRemainder {
			match = segment
		}
	}
	if match == nil {
		return nil, segmentStrings
	}
	if len(match.subPathSegments) == 0 {
		return match, segmentStrings[1:]
	}
	subMatch, leftStrings := findPathSegment(match.subPathSegments, method, host, segmentStrings[1:])
	if subMatch != nil {
		return subMatch, leftStrings
	}
	return match, segmentStrings[1:]
}

// NewServeMux allocates and returns a new ServeMux.
func NewServeMux() *ServeMux {
	return &ServeMux{
		pathRootSegments: make([]*muxPathSegment, 0),
		notFoundHandler: newAnonymousHandler(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}),
	}
}

// Handle registers the handler for the given pattern. If the given pattern conflicts with one that is already registered or if the pattern is invalid, Handle panics.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
	method, host, segmentsStrings, err := parsePattern(pattern)
	if err != nil {
		panic(fmt.Errorf("cloud not register handler: %w", err))
	}
	matchedSegment, remainingSegmentStrings := findPathSegment(mux.pathRootSegments, method, host, segmentsStrings)
	currentSegment := matchedSegment
	for _, segmentString := range remainingSegmentStrings[:len(remainingSegmentStrings)-1] {
		segment := muxPathSegment{
			method:          method,
			host:            host,
			pattern:         "",
			handler:         nil,
			segmentName:     segmentOrEmptyIfWildcard(segmentString),
			matchRemainder:  true,
			subPathSegments: make([]*muxPathSegment, 0),
		}
		if currentSegment != nil {
			currentSegment.subPathSegments = append(currentSegment.subPathSegments, &segment)
		} else {
			mux.pathRootSegments = append(mux.pathRootSegments, &segment)
		}
		currentSegment = &segment
	}
	segment := muxPathSegment{
		method:          method,
		host:            host,
		pattern:         pattern,
		handler:         &handler,
		segmentName:     segmentOrEmptyIfWildcard(segmentsStrings[len(segmentsStrings)-1]),
		matchRemainder:  strings.HasSuffix(pattern, "...}"),
		subPathSegments: make([]*muxPathSegment, 0),
	}
	if currentSegment == nil {
		mux.pathRootSegments = append(mux.pathRootSegments, &segment)
		return
	}
	if currentSegment.method != "" && currentSegment.method == segment.method {
		panic(fmt.Errorf("could not register conflicting handler"))
	}
	for _, subSegment := range currentSegment.subPathSegments {
		if subSegment.segmentName == segment.segmentName {
			panic(fmt.Errorf("cloud not register conflicting handler"))
		}
	}
	currentSegment.subPathSegments = append(currentSegment.subPathSegments, &segment)
}

// HandleFunc registers the handler function for the given pattern. If the given pattern conflicts with one that is already registered or if the pattern is invalid, HandleFunc panics.
func (mux *ServeMux) HandleFunc(pattern string, handler HandlerFunc) {
	mux.Handle(pattern, newAnonymousHandler(handler))
}

/*
Handler returns the handler to use for the given request, consulting r.Method, r.Host, and r.URL.Path. It always returns a non-nil handler. If the path is not in its canonical form, the handler will be an internally-generated handler that redirects to the canonical path. If the host contains a port, it is ignored when matching handlers.

The path and host are used unchanged for CONNECT requests.

Handler also returns the registered pattern that matches the request or, in the case of internally-generated redirects, the path that will match after following the redirect.

If there is no registered handler that applies to the request, Handler returns a “page not found” or “method not supported” handler and an empty pattern.

Handler does not modify its argument. In particular, it does not populate named path wildcards, so r.PathValue will always return the empty string.
*/
func (mux *ServeMux) Handler(r *http.Request) (h Handler, pattern string) {
	path := r.URL.Path
	pathSplits := strings.Split(strings.ToLower(path), "/")
	pathSplits[len(pathSplits)-1] = strings.Split(pathSplits[len(pathSplits)-1], "?")[0]
	segment, _ := findPathSegment(mux.pathRootSegments,
		strings.ToUpper(r.Method),
		strings.ToLower(r.Host),
		pathSplits[1:])
	if segment == nil {
		return mux.notFoundHandler, ""
	}
	return *segment.handler, segment.pattern
}

// ServeHTTP dispatches the request to the handler whose pattern most closely matches the request URL.
func (mux *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler, pattern := mux.Handler(r)
	r.Pattern = pattern
	pathSplits := strings.Split(r.URL.Path, "/")
	patternSplits := strings.Split(pattern, "/")
	for i, pat := range patternSplits {
		if !isSegmentWildcard(pat) {
			continue
		}
		pat = strings.TrimPrefix(strings.TrimSuffix(pat, "}"), "{")
		pat, matchRemainder := strings.CutSuffix(pat, "...")
		if matchRemainder {
			r.SetPathValue(pat, strings.Join(pathSplits[i:], "/"))
		} else {
			r.SetPathValue(pat, strings.Split(pathSplits[i], "?")[0])
		}
	}
	handler.ServeHTTP(w, r)
}
