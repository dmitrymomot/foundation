package gokit

// Radix tree implementation based on the original work by
// Armon Dadgar in https://github.com/armon/go-radix/blob/master/radix.go
// (MIT licensed). Heavily modified for use as a HTTP routing tree.

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type methodTyp uint

const (
	mSTUB methodTyp = 1 << iota
	mCONNECT
	mDELETE
	mGET
	mHEAD
	mOPTIONS
	mPATCH
	mPOST
	mPUT
	mTRACE
)

var mALL = mCONNECT | mDELETE | mGET | mHEAD |
	mOPTIONS | mPATCH | mPOST | mPUT | mTRACE

var methodMap = map[string]methodTyp{
	http.MethodConnect: mCONNECT,
	http.MethodDelete:  mDELETE,
	http.MethodGet:     mGET,
	http.MethodHead:    mHEAD,
	http.MethodOptions: mOPTIONS,
	http.MethodPatch:   mPATCH,
	http.MethodPost:    mPOST,
	http.MethodPut:     mPUT,
	http.MethodTrace:   mTRACE,
}

var reverseMethodMap = map[methodTyp]string{
	mCONNECT: http.MethodConnect,
	mDELETE:  http.MethodDelete,
	mGET:     http.MethodGet,
	mHEAD:    http.MethodHead,
	mOPTIONS: http.MethodOptions,
	mPATCH:   http.MethodPatch,
	mPOST:    http.MethodPost,
	mPUT:     http.MethodPut,
	mTRACE:   http.MethodTrace,
}

// routeParams holds URL parameters extracted from the route.
type routeParams struct {
	Keys   []string
	Values []string
}

type nodeTyp uint8

const (
	ntStatic   nodeTyp = iota // /home
	ntRegexp                  // /{id:[0-9]+}
	ntParam                   // /{user}
	ntCatchAll                // /api/v1/*
)

type node[C Context] struct {
	// subroutes on the leaf node
	subroutes Router[C]

	// regexp matcher for regexp nodes
	rex *regexp.Regexp

	// HTTP handler endpoints on the leaf node
	endpoints endpoints[C]

	// prefix is the common prefix we ignore
	prefix string

	// child nodes should be stored in-order for iteration,
	// in groups of the node type.
	children [ntCatchAll + 1]nodes[C]

	// first byte of the child prefix
	tail byte

	// node type: static, regexp, param, catchAll
	typ nodeTyp

	// first byte of the prefix
	label byte
}

// endpoints is a mapping of http method constants to handlers
// for a given route.
type endpoints[C Context] map[methodTyp]*endpoint[C]

type endpoint[C Context] struct {
	// endpoint handler
	handler HandlerFunc[C]

	// pattern is the routing pattern for handler nodes
	pattern string

	// parameter keys recorded on handler nodes
	paramKeys []string
}

func (s endpoints[C]) value(method methodTyp) *endpoint[C] {
	mh, ok := s[method]
	if !ok {
		mh = &endpoint[C]{}
		s[method] = mh
	}
	return mh
}

func (n *node[C]) insertRoute(method methodTyp, pattern string, handler HandlerFunc[C]) *node[C] {
	var parent *node[C]
	search := pattern

	for {
		// Handle key exhaustion
		if len(search) == 0 {
			// Insert or update the node's leaf handler
			n.setEndpoint(method, handler, pattern)
			return n
		}

		// We're going to be searching for a wild node next,
		// in this case, we need to get the tail
		var label = search[0]
		var segTail byte
		var segEndIdx int
		var segTyp nodeTyp
		var segRexpat string
		if label == '{' || label == '*' {
			segTyp, _, segRexpat, segTail, _, segEndIdx = patNextSegment(search)
		}

		var prefix string
		if segTyp == ntRegexp {
			prefix = segRexpat
		}

		// Look for the edge to attach to
		parent = n
		n = n.getEdge(segTyp, label, segTail, prefix)

		// No edge, create one
		if n == nil {
			child := &node[C]{label: label, tail: segTail, prefix: search}
			hn := parent.addChild(child, search)
			hn.setEndpoint(method, handler, pattern)

			return hn
		}

		// Found an edge to match the pattern

		if n.typ > ntStatic {
			// We found a param node, trim the param from the search path and continue.
			// This param/wild pattern segment would already be on the tree from a previous
			// call to addChild when creating a new node.
			search = search[segEndIdx:]
			continue
		}

		// Static nodes fall below here.
		// Determine longest prefix of the search key on match.
		commonPrefix := longestPrefix(search, n.prefix)
		if commonPrefix == len(n.prefix) {
			// the common prefix is as long as the current node's prefix we're attempting to insert.
			// keep the search going.
			search = search[commonPrefix:]
			continue
		}

		// Split the node
		child := &node[C]{
			typ:    ntStatic,
			prefix: search[:commonPrefix],
		}
		parent.replaceChild(search[0], segTail, child)

		// Restore the existing node
		n.label = n.prefix[commonPrefix]
		n.prefix = n.prefix[commonPrefix:]
		child.addChild(n, n.prefix)

		// If the new key is a subset, set the method/handler on this node and finish.
		search = search[commonPrefix:]
		if len(search) == 0 {
			child.setEndpoint(method, handler, pattern)
			return child
		}

		// Create a new edge for the node
		subchild := &node[C]{
			typ:    ntStatic,
			label:  search[0],
			prefix: search,
		}
		hn := child.addChild(subchild, search)
		hn.setEndpoint(method, handler, pattern)
		return hn
	}
}

// addChild appends the new `child` node to the tree using the `pattern` as the trie key.
func (n *node[C]) addChild(child *node[C], prefix string) *node[C] {
	search := prefix

	// handler leaf node added to the tree is the child.
	// this may be overridden later down the flow
	hn := child

	// Parse next segment
	segTyp, _, segRexpat, segTail, segStartIdx, segEndIdx := patNextSegment(search)

	// Add child depending on next up segment
	switch segTyp {

	case ntStatic:
		// Search prefix is all static (that is, has no params in path)
		// noop

	default:
		// Search prefix contains a param, regexp or wildcard

		if segTyp == ntRegexp {
			rex, err := regexp.Compile(segRexpat)
			if err != nil {
				panic(fmt.Errorf("%w: '%s'", ErrInvalidRegexp, segRexpat))
			}
			child.prefix = segRexpat
			child.rex = rex
		}

		if segStartIdx == 0 {
			// Route starts with a param
			child.typ = segTyp

			if segTyp == ntCatchAll {
				segStartIdx = -1
			} else {
				segStartIdx = segEndIdx
			}
			if segStartIdx < 0 {
				segStartIdx = len(search)
			}
			child.tail = segTail // for params, we set the tail

			if segStartIdx != len(search) {
				// add static edge for the remaining part, split the end.
				// its not possible to have adjacent param nodes, so its certainly
				// going to be a static node next.

				search = search[segStartIdx:] // advance search position

				nn := &node[C]{
					typ:    ntStatic,
					label:  search[0],
					prefix: search,
				}
				hn = child.addChild(nn, search)
			}

		} else if segStartIdx > 0 {
			// Route has some param

			// starts with a static segment
			child.typ = ntStatic
			child.prefix = search[:segStartIdx]
			child.rex = nil

			// add the param edge node
			search = search[segStartIdx:]

			nn := &node[C]{
				typ:   segTyp,
				label: search[0],
				tail:  segTail,
			}
			hn = child.addChild(nn, search)

		}
	}

	n.children[child.typ] = append(n.children[child.typ], child)
	n.children[child.typ].sort()
	return hn
}

func (n *node[C]) replaceChild(label, tail byte, child *node[C]) {
	for i := range n.children[child.typ] {
		if n.children[child.typ][i].label == label && n.children[child.typ][i].tail == tail {
			n.children[child.typ][i] = child
			n.children[child.typ][i].label = label
			n.children[child.typ][i].tail = tail
			return
		}
	}
	panic(ErrMissingChild)
}

func (n *node[C]) getEdge(ntyp nodeTyp, label, tail byte, prefix string) *node[C] {
	nds := n.children[ntyp]
	for i := range nds {
		if nds[i].label == label && nds[i].tail == tail {
			if ntyp == ntRegexp && nds[i].prefix != prefix {
				continue
			}
			return nds[i]
		}
	}
	return nil
}

func (n *node[C]) setEndpoint(method methodTyp, handler HandlerFunc[C], pattern string) {
	// Set the handler for the method type on the node
	if n.endpoints == nil {
		n.endpoints = make(endpoints[C])
	}

	paramKeys := patParamKeys(pattern)

	if method&mSTUB == mSTUB {
		n.endpoints.value(mSTUB).handler = handler
	}
	if method&mALL == mALL {
		h := n.endpoints.value(mALL)
		h.handler = handler
		h.pattern = pattern
		h.paramKeys = paramKeys
		for _, m := range methodMap {
			h := n.endpoints.value(m)
			h.handler = handler
			h.pattern = pattern
			h.paramKeys = paramKeys
		}
	} else {
		h := n.endpoints.value(method)
		h.handler = handler
		h.pattern = pattern
		h.paramKeys = paramKeys
	}
}

func (n *node[C]) findRoute(method methodTyp, path string) (*node[C], endpoints[C], HandlerFunc[C], routeParams) {
	// Reset the context routing pattern and params
	rctx := &routeParams{
		Keys:   make([]string, 0),
		Values: make([]string, 0),
	}

	// Find the routing handlers for the path
	rn := n.findRouteRecursive(method, path, rctx)
	if rn == nil {
		return nil, nil, nil, *rctx
	}

	// Record the routing pattern in the request lifecycle
	if rn.endpoints[method] != nil && rn.endpoints[method].handler != nil {
		return rn, rn.endpoints, rn.endpoints[method].handler, *rctx
	}

	return rn, rn.endpoints, nil, *rctx
}

// Recursive edge traversal by checking all nodeTyp groups along the way.
func (n *node[C]) findRouteRecursive(method methodTyp, path string, rctx *routeParams) *node[C] {
	nn := n
	search := path

	for t, nds := range nn.children {
		ntyp := nodeTyp(t)
		if len(nds) == 0 {
			continue
		}

		var xn *node[C]
		xsearch := search

		var label byte
		if search != "" {
			label = search[0]
		}

		switch ntyp {
		case ntStatic:
			xn = nds.findEdge(label)
			if xn == nil || !strings.HasPrefix(xsearch, xn.prefix) {
				continue
			}
			xsearch = xsearch[len(xn.prefix):]

		case ntParam, ntRegexp:
			// short-circuit and return no matching route for empty param values
			if xsearch == "" {
				continue
			}

			// serially loop through each node grouped by the tail delimiter
			for idx := range nds {
				xn = nds[idx]

				// label for param nodes is the delimiter byte
				p := strings.IndexByte(xsearch, xn.tail)

				if p < 0 {
					if xn.tail == '/' {
						p = len(xsearch)
					} else {
						continue
					}
				} else if ntyp == ntRegexp && p == 0 {
					continue
				}

				if ntyp == ntRegexp && xn.rex != nil {
					if !xn.rex.MatchString(xsearch[:p]) {
						continue
					}
				} else if strings.IndexByte(xsearch[:p], '/') != -1 {
					// avoid a match across path segments
					continue
				}

				prevlen := len(rctx.Values)
				rctx.Values = append(rctx.Values, xsearch[:p])
				xsearch = xsearch[p:]

				if len(xsearch) == 0 {
					if xn.isLeaf() {
						h := xn.endpoints[method]
						if h != nil && h.handler != nil {
							rctx.Keys = append(rctx.Keys, h.paramKeys...)
							return xn
						}

						// flag that the routing context found a route, but not a corresponding
						// supported method
						return xn
					}
				}

				// recursively find the next node on this branch
				fin := xn.findRouteRecursive(method, xsearch, rctx)
				if fin != nil {
					return fin
				}

				// not found on this branch, reset vars
				rctx.Values = rctx.Values[:prevlen]
				xsearch = search
			}

			rctx.Values = append(rctx.Values, "")

		default:
			// catch-all nodes
			rctx.Values = append(rctx.Values, search)
			xn = nds[0]
			xsearch = ""
		}

		if xn == nil {
			continue
		}

		// did we find it yet?
		if len(xsearch) == 0 {
			if xn.isLeaf() {
				h := xn.endpoints[method]
				if h != nil && h.handler != nil {
					rctx.Keys = append(rctx.Keys, h.paramKeys...)
					return xn
				}

				// flag that the routing context found a route, but not a corresponding
				// supported method
				return xn
			}
		}

		// recursively find the next node..
		fin := xn.findRouteRecursive(method, xsearch, rctx)
		if fin != nil {
			return fin
		}

		// Did not find final handler, let's remove the param here if it was set
		if xn.typ > ntStatic {
			if len(rctx.Values) > 0 {
				rctx.Values = rctx.Values[:len(rctx.Values)-1]
			}
		}

	}

	return nil
}

func (n *node[C]) isLeaf() bool {
	return n.endpoints != nil
}

func (n *node[C]) routes() []Route {
	rts := []Route{}

	n.walk(func(eps endpoints[C], subroutes Router[C]) bool {
		if eps[mSTUB] != nil && eps[mSTUB].handler != nil && subroutes == nil {
			return false
		}

		// Group methodHandlers by unique patterns
		pats := make(map[string]endpoints[C])

		for mt, h := range eps {
			if h.pattern == "" {
				continue
			}
			p, ok := pats[h.pattern]
			if !ok {
				p = endpoints[C]{}
				pats[h.pattern] = p
			}
			p[mt] = h
		}

		for p, mh := range pats {
			for mt := range mh {
				if mt == mALL || mt == mSTUB {
					continue
				}
				m := methodTypString(mt)
				if m == "" {
					continue
				}
				rt := Route{Method: m, Pattern: p}
				rts = append(rts, rt)
			}
		}

		return false
	})

	return rts
}

func (n *node[C]) walk(fn func(eps endpoints[C], subroutes Router[C]) bool) bool {
	// Visit the leaf values if any
	if (n.endpoints != nil || n.subroutes != nil) && fn(n.endpoints, n.subroutes) {
		return true
	}

	// Recurse on the children
	for _, ns := range n.children {
		for _, cn := range ns {
			if cn.walk(fn) {
				return true
			}
		}
	}
	return false
}

// patNextSegment returns the next segment details from a pattern:
// node type, param key, regexp string, param tail byte, param starting index, param ending index
func patNextSegment(pattern string) (nodeTyp, string, string, byte, int, int) {
	ps := strings.Index(pattern, "{")
	ws := strings.Index(pattern, "*")

	if ps < 0 && ws < 0 {
		return ntStatic, "", "", 0, 0, len(pattern) // we return the entire thing
	}

	// Sanity check
	if ps >= 0 && ws >= 0 && ws < ps {
		panic(ErrWildcardPosition)
	}

	var tail byte = '/' // Default endpoint tail to / byte

	if ps >= 0 {
		// Param/Regexp pattern is next
		nt := ntParam

		// Read to closing } taking into account opens and closes in curl count (cc)
		cc := 0
		pe := ps
		for i, c := range pattern[ps:] {
			if c == '{' {
				cc++
			} else if c == '}' {
				cc--
				if cc == 0 {
					pe = ps + i
					break
				}
			}
		}
		if pe == ps {
			panic(ErrParamDelimiter)
		}

		key := pattern[ps+1 : pe]
		pe++ // set end to next position

		if pe < len(pattern) {
			tail = pattern[pe]
		}

		key, rexpat, isRegexp := strings.Cut(key, ":")
		if isRegexp {
			nt = ntRegexp
		}

		if len(rexpat) > 0 {
			if rexpat[0] != '^' {
				rexpat = "^" + rexpat
			}
			if rexpat[len(rexpat)-1] != '$' {
				rexpat += "$"
			}
		}

		return nt, key, rexpat, tail, ps, pe
	}

	// Wildcard pattern as finale
	if ws < len(pattern)-1 {
		panic(ErrWildcardPosition)
	}
	return ntCatchAll, "*", "", 0, ws, len(pattern)
}

func patParamKeys(pattern string) []string {
	pat := pattern
	paramKeys := []string{}
	for {
		ptyp, paramKey, _, _, _, e := patNextSegment(pat)
		if ptyp == ntStatic {
			return paramKeys
		}
		for i := range paramKeys {
			if paramKeys[i] == paramKey {
				panic(fmt.Errorf("%w: '%s' has duplicate key '%s'", ErrDuplicateParam, pattern, paramKey))
			}
		}
		paramKeys = append(paramKeys, paramKey)
		pat = pat[e:]
	}
}

// longestPrefix finds the length of the shared prefix
// of two strings
func longestPrefix(k1, k2 string) int {
	max := len(k1)
	if l := len(k2); l < max {
		max = l
	}
	var i int
	for i = 0; i < max; i++ {
		if k1[i] != k2[i] {
			break
		}
	}
	return i
}

func methodTypString(method methodTyp) string {
	for s, t := range methodMap {
		if method == t {
			return s
		}
	}
	return ""
}

type nodes[C Context] []*node[C]

// sort the list of nodes by label
func (ns nodes[C]) sort()              { sort.Sort(ns); ns.tailSort() }
func (ns nodes[C]) Len() int           { return len(ns) }
func (ns nodes[C]) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns nodes[C]) Less(i, j int) bool { return ns[i].label < ns[j].label }

// tailSort pushes nodes with '/' as the tail to the end of the list for param nodes.
// The list order determines the traversal order.
func (ns nodes[C]) tailSort() {
	for i := len(ns) - 1; i >= 0; i-- {
		if ns[i].typ > ntStatic && ns[i].tail == '/' {
			ns.Swap(i, len(ns)-1)
			return
		}
	}
}

func (ns nodes[C]) findEdge(label byte) *node[C] {
	num := len(ns)
	idx := 0
	i, j := 0, num-1
	for i <= j {
		idx = i + (j-i)/2
		if label > ns[idx].label {
			i = idx + 1
		} else if label < ns[idx].label {
			j = idx - 1
		} else {
			i = num // breaks cond
		}
	}
	if ns[idx].label != label {
		return nil
	}
	return ns[idx]
}
