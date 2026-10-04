package tmplfuncs

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"text/template/parse"
)

// MaxOutput bounds what one render of a user-authored template may write.
const MaxOutput = 1 << 20

// MaxRangeDepth bounds how deeply {{range}} blocks may nest.
const MaxRangeDepth = 2

// ErrOutputLimit is returned when a render passes MaxOutput.
var ErrOutputLimit = errors.New("template output exceeds the size limit")

var (
	errRangeSource = errors.New("range only iterates over a field, like {{range .items}}")
	errRangeDepth  = fmt.Errorf("range blocks may nest at most %d deep", MaxRangeDepth)
	errTemplateUse = errors.New("{{template}} and {{block}} are not supported")
	errRootAssign  = errors.New("$ cannot be reassigned")
)

// Compile parses a user-authored template with the shared funcs and absent keys
// rendering as their zero value, refusing it when CheckLimits does.
func Compile(name, src string) (*template.Template, error) {
	t, err := template.New(name).Funcs(funcs).Option("missingkey=zero").Parse(src)
	if err != nil {
		return nil, err
	}
	if err := CheckLimits(t); err != nil {
		return nil, err
	}
	return t, nil
}

// CheckLimits refuses constructs whose cost is not bounded by the data: a
// {{range}} over anything but a field of the data, {{range}} nested past
// MaxRangeDepth, {{template}} calls, and reassigning $.
func CheckLimits(t *template.Template) error {
	for _, tt := range t.Templates() {
		if tt.Tree == nil || tt.Root == nil {
			continue
		}
		if err := checkNode(tt.Root, 0, true); err != nil {
			return err
		}
	}
	return nil
}

// checkNode walks n; dotIsData is whether dot can only hold a value from the data.
func checkNode(n parse.Node, depth int, dotIsData bool) error {
	switch x := n.(type) {
	case *parse.ListNode:
		if x == nil {
			return nil
		}
		for _, c := range x.Nodes {
			if err := checkNode(c, depth, dotIsData); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return checkPipe(x.Pipe)
	case *parse.RangeNode:
		if err := checkPipe(x.Pipe); err != nil {
			return err
		}
		if depth+1 > MaxRangeDepth {
			return errRangeDepth
		}
		if !isDataPath(x.Pipe, dotIsData) {
			return errRangeSource
		}
		if err := checkNode(x.List, depth+1, true); err != nil {
			return err
		}
		return checkNode(x.ElseList, depth, dotIsData)
	case *parse.IfNode:
		if err := checkPipe(x.Pipe); err != nil {
			return err
		}
		if err := checkNode(x.List, depth, dotIsData); err != nil {
			return err
		}
		return checkNode(x.ElseList, depth, dotIsData)
	case *parse.WithNode:
		if err := checkPipe(x.Pipe); err != nil {
			return err
		}
		if err := checkNode(x.List, depth, isDataPath(x.Pipe, dotIsData)); err != nil {
			return err
		}
		return checkNode(x.ElseList, depth, dotIsData)
	case *parse.TemplateNode:
		return errTemplateUse
	}
	return nil
}

// checkPipe refuses a pipeline that declares or assigns $, the data root.
func checkPipe(p *parse.PipeNode) error {
	if p == nil {
		return nil
	}
	for _, v := range p.Decl {
		if len(v.Ident) > 0 && v.Ident[0] == "$" {
			return errRootAssign
		}
	}
	for _, c := range p.Cmds {
		for _, a := range c.Args {
			if sub, ok := a.(*parse.PipeNode); ok {
				if err := checkPipe(sub); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// isDataPath reports whether a pipeline is a single reference into the data
// (.x, .x.y, $.x, $v.x), so ranging over it iterates at most the data's length.
func isDataPath(p *parse.PipeNode, dotIsData bool) bool {
	if p == nil || len(p.Cmds) != 1 || len(p.Cmds[0].Args) != 1 {
		return false
	}
	return isPathNode(p.Cmds[0].Args[0], dotIsData)
}

func isPathNode(n parse.Node, dotIsData bool) bool {
	switch x := n.(type) {
	case *parse.DotNode, *parse.FieldNode:
		return dotIsData
	case *parse.VariableNode:
		// A bare variable may hold a literal; one indexed into must be a map.
		return x.Ident[0] == "$" || len(x.Ident) > 1
	case *parse.ChainNode:
		return isPathNode(x.Node, dotIsData)
	}
	return false
}

// Execute renders t into a string, failing once the output passes MaxOutput.
func Execute(t *template.Template, data any) (string, error) {
	w := &cappedWriter{max: MaxOutput}
	if err := t.Execute(w, data); err != nil {
		return "", err
	}
	return w.b.String(), nil
}

type cappedWriter struct {
	b   strings.Builder
	max int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.b.Len()+len(p) > w.max {
		return 0, ErrOutputLimit
	}
	return w.b.Write(p)
}

// CacheCap bounds how many compiled templates a Cache keeps.
const CacheCap = 4096

// Cache maps a template source to its compiled form, nil marking a source
// that failed to compile. Past CacheCap a miss is recompiled, not stored.
type Cache struct {
	m sync.Map
	n atomic.Int64
}

// Load returns the cached template for src and whether src was cached.
func (c *Cache) Load(src string) (*template.Template, bool) {
	v, ok := c.m.Load(src)
	if !ok {
		return nil, false
	}
	t, _ := v.(*template.Template)
	return t, true
}

// Store caches t (nil for a known-bad source) unless the cache is full.
func (c *Cache) Store(src string, t *template.Template) {
	if c.n.Load() >= CacheCap {
		return
	}
	if _, loaded := c.m.LoadOrStore(src, t); !loaded {
		c.n.Add(1)
	}
}
