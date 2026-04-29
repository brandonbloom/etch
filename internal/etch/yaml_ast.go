package etch

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

type yamlAliasSemantic struct {
	Name string
}

type yamlTagSemantic struct {
	Tag   string
	Value any
}

func parseYAMLFile(raw []byte) (*ast.File, error) {
	if strings.TrimSpace(string(raw)) == "" {
		node, err := newYAMLValueNode(map[string]any{})
		if err != nil {
			return nil, err
		}
		return &ast.File{Docs: []*ast.DocumentNode{ast.Document(nil, node)}}, nil
	}
	return parser.ParseBytes(raw, parser.ParseComments)
}

func mutateYAMLFile(file *ast.File, selector, verb string, value any) (bool, error) {
	parts, err := ParseSelector(selector)
	if err != nil {
		return false, err
	}
	doc, err := firstYAMLDocument(file)
	if err != nil {
		return false, err
	}
	if doc.Body == nil {
		doc.Body, err = newYAMLValueNode(map[string]any{})
		if err != nil {
			return false, err
		}
	} else if comments, ok := doc.Body.(*ast.CommentGroupNode); ok {
		doc.Body, err = newYAMLValueNode(map[string]any{})
		if err != nil {
			return false, err
		}
		_ = doc.Body.SetComment(comments)
	}
	if len(parts) == 0 {
		return mutateYAMLRoot(doc, verb, value)
	}
	if verb == "set" {
		// Let go-yaml handle ordinary existing-node replacement. The local walker
		// remains necessary for missing containers and concrete anchor/tag nodes.
		changed, found, err := setExistingYAMLPath(doc, parts, value)
		if err != nil {
			return false, err
		}
		if found {
			return changed, nil
		}
	}
	return mutateYAMLDesc(&doc.Body, parts, verb, value)
}

func firstYAMLDocument(file *ast.File) (*ast.DocumentNode, error) {
	if len(file.Docs) == 0 {
		node, err := newYAMLValueNode(map[string]any{})
		if err != nil {
			return nil, err
		}
		file.Docs = append(file.Docs, ast.Document(nil, node))
	}
	return file.Docs[0], nil
}

func firstYAMLDocumentString(file *ast.File) string {
	doc, err := firstYAMLDocument(file)
	if err != nil || doc.Body == nil {
		return ""
	}
	return doc.Body.String()
}

func mutateYAMLRoot(doc *ast.DocumentNode, verb string, value any) (bool, error) {
	switch verb {
	case "set":
		if yamlSemanticEqualNodeValue(doc.Body, value) {
			return false, nil
		}
		next, err := newYAMLValueNode(value)
		if err != nil {
			return false, err
		}
		if doc.Body != nil {
			placeYAMLNodeLike(next, doc.Body)
			copyYAMLComment(next, doc.Body)
		}
		doc.Body = next
		return true, nil
	case "delete", "remove":
		return false, nil
	case "append", "add":
		seq, err := yamlSequenceNode(doc.Body)
		if err != nil {
			return false, failf("selector $ does not identify an array")
		}
		return mutateYAMLSequenceSelected(seq, verb, value)
	default:
		return false, usagef("unknown structured verb %s", verb)
	}
}

func mutateYAMLDesc(nodep *ast.Node, parts []selectorPart, verb string, value any) (bool, error) {
	switch node := (*nodep).(type) {
	case *ast.AnchorNode:
		return mutateYAMLDesc(&node.Value, parts, verb, value)
	case *ast.TagNode:
		return mutateYAMLDesc(&node.Value, parts, verb, value)
	case *ast.AliasNode:
		return false, failf("selector intermediate is an alias")
	}

	p := parts[0]
	last := len(parts) == 1
	if p.IsKey {
		m, err := yamlMappingNode(*nodep)
		if err != nil {
			return false, failf("selector component %s found %T, want object", p.Key, *nodep)
		}
		if last {
			return mutateYAMLMapLeaf(m, p.Key, verb, value)
		}
		mv, _, err := findYAMLMapValue(m, p.Key)
		if err != nil {
			return false, err
		}
		if mv == nil {
			child, err := newYAMLContainerNode(parts[1])
			if err != nil {
				return false, err
			}
			mv, err = appendYAMLMapValue(m, p.Key, child)
			if err != nil {
				return false, err
			}
		}
		return mutateYAMLDesc(&mv.Value, parts[1:], verb, value)
	}

	seq, err := yamlSequenceNode(*nodep)
	if err != nil {
		return false, failf("selector component [%d] found %T, want array", p.Index, *nodep)
	}
	if p.Index > len(seq.Values) || (!last && p.Index == len(seq.Values)) {
		return false, failf("array index %d out of range", p.Index)
	}
	if last {
		return mutateYAMLArrayLeaf(seq, p.Index, verb, value)
	}
	return mutateYAMLDesc(&seq.Values[p.Index], parts[1:], verb, value)
}

func mutateYAMLMapLeaf(m *ast.MappingNode, key string, verb string, value any) (bool, error) {
	mv, idx, err := findYAMLMapValue(m, key)
	if err != nil {
		return false, err
	}
	switch verb {
	case "set":
		if mv != nil {
			if yamlSemanticEqualNodeValue(mv.Value, value) {
				return false, nil
			}
			next, err := newYAMLValueNode(value)
			if err != nil {
				return false, err
			}
			if m.IsFlowStyle {
				setYAMLFlowStyle(next, true)
			}
			return true, replaceYAMLNode(&mv.Value, next)
		}
		next, err := newYAMLValueNode(value)
		if err != nil {
			return false, err
		}
		_, err = appendYAMLMapValue(m, key, next)
		return err == nil, err
	case "delete":
		if mv == nil {
			return false, nil
		}
		deleteYAMLMapValue(m, idx)
		return true, nil
	case "append", "add":
		var seq *ast.SequenceNode
		if mv == nil {
			seqNode, err := newYAMLValueNode([]any{})
			if err != nil {
				return false, err
			}
			appended, err := appendYAMLMapValue(m, key, seqNode)
			if err != nil {
				return false, err
			}
			mv = appended
			seq = seqNode.(*ast.SequenceNode)
		} else {
			seq, err = yamlSequenceNode(mv.Value)
			if err != nil {
				return false, failf("selector $.%s does not identify an array", key)
			}
		}
		return mutateYAMLSequenceSelected(seq, verb, value)
	case "remove":
		if mv == nil {
			return false, nil
		}
		seq, err := yamlSequenceNode(mv.Value)
		if err != nil {
			return false, failf("selector $.%s does not identify an array", key)
		}
		return mutateYAMLSequenceSelected(seq, verb, value)
	default:
		return false, usagef("unknown structured verb %s", verb)
	}
}

func mutateYAMLArrayLeaf(seq *ast.SequenceNode, idx int, verb string, value any) (bool, error) {
	switch verb {
	case "set":
		if idx == len(seq.Values) {
			next, err := newYAMLValueNode(value)
			if err != nil {
				return false, err
			}
			appendYAMLSequenceValue(seq, next)
			return true, nil
		}
		if idx > len(seq.Values) {
			return false, failf("array index %d out of range", idx)
		}
		if yamlSemanticEqualNodeValue(seq.Values[idx], value) {
			return false, nil
		}
		next, err := newYAMLValueNode(value)
		if err != nil {
			return false, err
		}
		if seq.IsFlowStyle {
			setYAMLFlowStyle(next, true)
		}
		return true, replaceYAMLNode(&seq.Values[idx], next)
	case "delete":
		if idx >= len(seq.Values) {
			return false, nil
		}
		removeYAMLSequenceIndex(seq, idx)
		return true, nil
	case "append", "add", "remove":
		if idx >= len(seq.Values) {
			return false, failf("array index %d out of range", idx)
		}
		nested, err := yamlSequenceNode(seq.Values[idx])
		if err != nil {
			return false, failf("selector component [%d] does not identify an array", idx)
		}
		return mutateYAMLSequenceSelected(nested, verb, value)
	default:
		return false, usagef("unknown structured verb %s", verb)
	}
}

func mutateYAMLSequenceSelected(seq *ast.SequenceNode, verb string, value any) (bool, error) {
	switch verb {
	case "append":
		next, err := newYAMLValueNode(value)
		if err != nil {
			return false, err
		}
		appendYAMLSequenceValue(seq, next)
		return true, nil
	case "add":
		for _, item := range seq.Values {
			if yamlSemanticEqualNodeValue(item, value) {
				return false, nil
			}
		}
		next, err := newYAMLValueNode(value)
		if err != nil {
			return false, err
		}
		appendYAMLSequenceValue(seq, next)
		return true, nil
	case "remove":
		changed := false
		for i := 0; i < len(seq.Values); {
			if yamlSemanticEqualNodeValue(seq.Values[i], value) {
				removeYAMLSequenceIndex(seq, i)
				changed = true
				continue
			}
			i++
		}
		return changed, nil
	default:
		return false, usagef("unknown structured verb %s", verb)
	}
}

func deleteYAMLMapValue(m *ast.MappingNode, idx int) {
	deleted := m.Values[idx]
	m.Values = append(m.Values[:idx], m.Values[idx+1:]...)
	// A leading document/comment block may be attached to the deleted key; keep
	// it with the following key when one exists.
	if deleted.GetComment() == nil || idx >= len(m.Values) || m.Values[idx].GetComment() != nil {
		return
	}
	_ = m.Values[idx].SetComment(deleted.GetComment())
}

func setExistingYAMLPath(doc *ast.DocumentNode, parts []selectorPart, value any) (changed bool, found bool, err error) {
	path, err := yamlPathForSelectorParts(parts)
	if err != nil {
		return false, false, err
	}
	tmp := &ast.File{Docs: []*ast.DocumentNode{doc}}
	old, err := path.FilterFile(tmp)
	if err != nil {
		if yaml.IsNotFoundNodeError(err) {
			return false, false, nil
		}
		// go-yaml's path evaluator treats anchors/tags as opaque for descent.
		// Falling back lets etch mutate the concrete node representation.
		return false, false, nil
	}
	if yamlSemanticEqualNodeValue(old, value) {
		return false, true, nil
	}
	next, err := newYAMLValueNode(value)
	if err != nil {
		return false, false, err
	}
	copyYAMLComment(next, old)
	if yamlNodeFlowParent(doc.Body, old) {
		setYAMLFlowStyle(next, true)
	}
	if err := path.ReplaceWithNode(tmp, next); err != nil {
		return false, false, err
	}
	return true, true, nil
}

func yamlPathForSelectorParts(parts []selectorPart) (*yaml.Path, error) {
	var b strings.Builder
	b.WriteByte('$')
	for _, part := range parts {
		if part.IsKey {
			b.WriteString(".'")
			b.WriteString(strings.ReplaceAll(strings.ReplaceAll(part.Key, `\`, `\\`), `'`, `\'`))
			b.WriteByte('\'')
			continue
		}
		fmt.Fprintf(&b, "[%d]", part.Index)
	}
	return yaml.PathString(b.String())
}

func yamlNodeFlowParent(root, child ast.Node) bool {
	switch parent := ast.Parent(root, child).(type) {
	case *ast.MappingValueNode:
		return parent.IsFlowStyle
	case *ast.MappingNode:
		return parent.IsFlowStyle
	case *ast.SequenceNode:
		return parent.IsFlowStyle
	default:
		return false
	}
}

func findYAMLMapValue(m *ast.MappingNode, key string) (*ast.MappingValueNode, int, error) {
	for i, mv := range m.Values {
		got, err := yamlMapKeyString(mv.Key)
		if err != nil {
			return nil, 0, err
		}
		if got == key {
			return mv, i, nil
		}
	}
	return nil, -1, nil
}

func yamlMapKeyString(key ast.MapKeyNode) (string, error) {
	if key == nil || key.GetToken() == nil {
		return "", failf("unsupported empty YAML mapping key")
	}
	if key.IsMergeKey() {
		return key.GetToken().Value, nil
	}
	if scalar, ok := key.(ast.ScalarNode); ok {
		return fmt.Sprint(scalar.GetValue()), nil
	}
	if tag, ok := key.(*ast.TagNode); ok {
		return fmt.Sprint(tag.GetValue()), nil
	}
	return key.GetToken().Value, nil
}

func yamlMappingNode(node ast.Node) (*ast.MappingNode, error) {
	switch n := node.(type) {
	case *ast.MappingNode:
		return n, nil
	case *ast.AnchorNode:
		return yamlMappingNode(n.Value)
	case *ast.TagNode:
		return yamlMappingNode(n.Value)
	case *ast.AliasNode:
		return nil, failf("selector intermediate is an alias")
	default:
		return nil, failf("expected object")
	}
}

func yamlSequenceNode(node ast.Node) (*ast.SequenceNode, error) {
	switch n := node.(type) {
	case *ast.SequenceNode:
		return n, nil
	case *ast.AnchorNode:
		return yamlSequenceNode(n.Value)
	case *ast.TagNode:
		return yamlSequenceNode(n.Value)
	case *ast.AliasNode:
		return nil, failf("selector intermediate is an alias")
	default:
		return nil, failf("expected array")
	}
}

func newYAMLContainerNode(next selectorPart) (ast.Node, error) {
	if next.IsKey {
		return newYAMLValueNode(map[string]any{})
	}
	return newYAMLValueNode([]any{})
}

func newYAMLValueNode(value any) (ast.Node, error) {
	node, err := yaml.ValueToNode(value, yaml.UseLiteralStyleIfMultiline(true))
	if err != nil {
		return nil, err
	}
	setYAMLFlowStyle(node, false)
	return node, nil
}

func appendYAMLMapValue(m *ast.MappingNode, key string, value ast.Node) (*ast.MappingValueNode, error) {
	keyNode, err := newYAMLMapKeyNode(key)
	if err != nil {
		return nil, err
	}
	keyCol := yamlMapKeyColumn(m)
	placeYAMLNode(keyNode, keyCol)
	placeYAMLNode(value, yamlMapValueColumn(keyCol, value))
	if m.IsFlowStyle {
		setYAMLFlowStyle(value, true)
	}
	mv := ast.MappingValue(nil, keyNode, value)
	mv.IsFlowStyle = m.IsFlowStyle
	m.Values = append(m.Values, mv)
	return mv, nil
}

func newYAMLMapKeyNode(key string) (ast.MapKeyNode, error) {
	node, err := newYAMLValueNode(key)
	if err != nil {
		return nil, err
	}
	keyNode, ok := node.(ast.MapKeyNode)
	if !ok {
		return nil, failf("unsupported YAML mapping key %q", key)
	}
	return keyNode, nil
}

func appendYAMLSequenceValue(seq *ast.SequenceNode, value ast.Node) {
	if seq.IsFlowStyle {
		setYAMLFlowStyle(value, true)
		placeYAMLNode(value, yamlSequenceColumn(seq))
	} else {
		placeYAMLNode(value, yamlSequenceColumn(seq)+2)
	}
	seq.Values = append(seq.Values, value)
	if len(seq.ValueHeadComments) > 0 {
		seq.ValueHeadComments = append(seq.ValueHeadComments, nil)
	}
}

func removeYAMLSequenceIndex(seq *ast.SequenceNode, idx int) {
	seq.Values = append(seq.Values[:idx], seq.Values[idx+1:]...)
	if len(seq.ValueHeadComments) == len(seq.Values)+1 {
		seq.ValueHeadComments = append(seq.ValueHeadComments[:idx], seq.ValueHeadComments[idx+1:]...)
	}
	if len(seq.Entries) == len(seq.Values)+1 {
		seq.Entries = append(seq.Entries[:idx], seq.Entries[idx+1:]...)
	}
}

func replaceYAMLNode(nodep *ast.Node, next ast.Node) error {
	old := *nodep
	placeYAMLNodeLike(next, old)
	copyYAMLComment(next, old)
	*nodep = next
	return nil
}

func copyYAMLComment(dst, src ast.Node) {
	if src == nil || dst == nil || dst.GetComment() != nil || src.GetComment() == nil {
		return
	}
	_ = dst.SetComment(src.GetComment())
}

func placeYAMLNodeLike(node, old ast.Node) {
	if old == nil || old.GetToken() == nil || old.GetToken().Position == nil {
		placeYAMLNode(node, 1)
		return
	}
	placeYAMLNode(node, old.GetToken().Position.Column)
}

func placeYAMLNode(node ast.Node, column int) {
	if node == nil || node.GetToken() == nil || node.GetToken().Position == nil {
		return
	}
	node.AddColumn(column - node.GetToken().Position.Column)
}

func yamlMapKeyColumn(m *ast.MappingNode) int {
	for _, mv := range m.Values {
		if mv.Key != nil && mv.Key.GetToken() != nil && mv.Key.GetToken().Position != nil {
			return mv.Key.GetToken().Position.Column
		}
	}
	if m.GetToken() != nil && m.GetToken().Position != nil && m.GetToken().Position.Column > 0 {
		return m.GetToken().Position.Column
	}
	return 1
}

func yamlMapValueColumn(keyCol int, value ast.Node) int {
	switch value.(type) {
	case *ast.MappingNode, *ast.SequenceNode:
		return keyCol + 2
	case *ast.AnchorNode, *ast.TagNode:
		return keyCol + 2
	}
	return keyCol
}

func yamlSequenceColumn(seq *ast.SequenceNode) int {
	if seq.GetToken() != nil && seq.GetToken().Position != nil && seq.GetToken().Position.Column > 0 {
		return seq.GetToken().Position.Column
	}
	return 1
}

func setYAMLFlowStyle(node ast.Node, flow bool) {
	switch n := node.(type) {
	case *ast.MappingNode:
		n.SetIsFlowStyle(flow)
	case *ast.SequenceNode:
		n.SetIsFlowStyle(flow)
	case *ast.AnchorNode:
		setYAMLFlowStyle(n.Value, flow)
	case *ast.TagNode:
		setYAMLFlowStyle(n.Value, flow)
	}
}

func yamlSemanticEqualNodeValue(node ast.Node, value any) bool {
	return reflect.DeepEqual(canonicalYAMLSemantic(yamlNodeSemantic(node)), canonicalYAMLSemantic(value))
}

func yamlNodeSemantic(node ast.Node) any {
	switch n := node.(type) {
	case *ast.MappingNode:
		m := make(map[string]any, len(n.Values))
		for _, mv := range n.Values {
			key, err := yamlMapKeyString(mv.Key)
			if err != nil {
				continue
			}
			m[key] = yamlNodeSemantic(mv.Value)
		}
		return m
	case *ast.MappingValueNode:
		return yamlNodeSemantic(n.Value)
	case *ast.SequenceNode:
		out := make([]any, 0, len(n.Values))
		for _, value := range n.Values {
			out = append(out, yamlNodeSemantic(value))
		}
		return out
	case *ast.AnchorNode:
		return yamlNodeSemantic(n.Value)
	case *ast.AliasNode:
		return yamlAliasSemantic{Name: fmt.Sprint(n.GetValue())}
	case *ast.TagNode:
		tag := ""
		if n.Start != nil {
			tag = n.Start.Value
		}
		return yamlTagSemantic{Tag: tag, Value: yamlNodeSemantic(n.Value)}
	case *ast.LiteralNode:
		if n.Value != nil {
			return n.Value.GetValue()
		}
		return n.GetValue()
	case ast.ScalarNode:
		return n.GetValue()
	default:
		return node.String()
	}
}

func canonicalYAMLSemantic(value any) any {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = canonicalYAMLSemantic(v[i])
		}
		return out
	case map[string]any:
		m := make(map[string]any, len(v))
		for k, item := range v {
			m[k] = canonicalYAMLSemantic(item)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(v))
		for k, item := range v {
			m[fmt.Sprint(k)] = canonicalYAMLSemantic(item)
		}
		return m
	case yamlTagSemantic:
		return yamlTagSemantic{Tag: v.Tag, Value: canonicalYAMLSemantic(v.Value)}
	default:
		return value
	}
}
