package dsl

import (
	"fmt"
	"regexp"
	"strings"
)

// dslRefPattern matches @label.table references.
// Label supports hyphens (common in env labels like "aiops-mysql").
// Table follows SQL identifier rules.
var dslRefPattern = regexp.MustCompile(`@([a-zA-Z_][a-zA-Z0-9_-]*)\.([a-zA-Z_][a-zA-Z0-9_]*)`)

// preprocess scans DSL input for @label.table and @label.promql(...) patterns
// and replaces them with unique placeholders (__dsl_N__). The placeholders are
// valid SQL identifiers that the sqlast parser can handle as table names.
//
// @label.promql(...) is handled first (pre-scan) because the expression inside
// can contain nested parentheses, brackets, and other characters that the regex
// cannot match. The extracted expression is stored in SourceRef.Table with
// IsRawPromQL=true, and the placeholder becomes a valid SQL identifier.
//
// The returned refs slice maps each placeholder to its original SourceRef.
func preprocess(input string) (string, []SourceRef, error) {
	var refs []SourceRef
	counter := 0
	seen := make(map[string]string)

	// ── Pass 1: handle @label.promql(...) references ──
	// These contain arbitrary PromQL expressions with nested parens/brackets
	// that cannot be handled by the simple regex pattern.
	result, promqlRefs, err := extractPromQLRefs(input)
	if err != nil {
		return "", nil, err
	}

	// Merge promql refs into the shared state
	for _, ref := range promqlRefs {
		key := "promql:" + ref.Label + ":" + ref.Table
		seen[key] = ref.Placeholder
		refs = append(refs, ref)
		counter++
	}

	// ── Pass 2: handle regular @label.table patterns via regex ──
	result = dslRefPattern.ReplaceAllStringFunc(result, func(match string) string {
		// match is "@label.table" — strip "@" and split on "."
		inner := match[1:] // skip '@'
		label, table, _ := strings.Cut(inner, ".")
		if label == "" || table == "" {
			return match
		}

		// Deduplicate identical references (same label + table)
		key := label + "." + table
		if existing, ok := seen[key]; ok {
			return existing
		}

		placeholder := fmt.Sprintf("__dsl_%d__", counter)
		counter++

		refs = append(refs, SourceRef{
			Label:       label,
			Table:       table,
			Placeholder: placeholder,
		})
		seen[key] = placeholder
		return placeholder
	})

	if counter == 0 {
		return result, nil, nil
	}

	return result, refs, nil
}

// extractPromQLRefs scans input for @label.promql(expr) patterns and
// replaces them with placeholders. It handles nested parentheses in the
// expression by tracking paren depth.
//
// Format: @labelname.promql(arbitrary PromQL expression with (nested parens))
//
// The expression is extracted as-is (including the outer parens content)
// and stored as the table part of SourceRef with IsRawPromQL=true.
func extractPromQLRefs(input string) (string, []SourceRef, error) {
	var refs []SourceRef
	var sb strings.Builder
	counter := 0

	i := 0
	for i < len(input) {
		if input[i] != '@' {
			sb.WriteByte(input[i])
			i++
			continue
		}

		// Found '@', try to parse @label.promql(...)
		j := i + 1
		labelStart := j
		for j < len(input) && (isIdentChar(input[j]) || input[j] == '-') {
			j++
		}
		label := input[labelStart:j]

		// Check for .promql( suffix
		if j == labelStart || j+8 > len(input) || input[j:j+8] != ".promql(" {
			// Not a promql() reference — emit '@' and continue;
			// the regex in Pass 2 will handle normal @label.table
			sb.WriteByte(input[i])
			i++
			continue
		}

		// Find matching closing paren (handles nested parens)
		parenDepth := 1
		k := j + 8 // position after '('
		exprStart := k
		for k < len(input) && parenDepth > 0 {
			switch input[k] {
			case '(':
				parenDepth++
			case ')':
				parenDepth--
			}
			k++
		}
		if parenDepth != 0 {
			return "", nil, fmt.Errorf("unclosed promql() in @%s.promql", label)
		}

		// Extract expression (content between outermost parens)
		expr := strings.TrimSpace(input[exprStart : k-1])
		if expr == "" {
			return "", nil, fmt.Errorf("empty promql() expression for @%s", label)
		}

		placeholder := fmt.Sprintf("__dsl_%d__", counter)
		counter++

		refs = append(refs, SourceRef{
			Label:       label,
			Table:       expr,
			Placeholder: placeholder,
			IsRawPromQL: true,
		})

		sb.WriteString(placeholder)
		i = k
	}

	return sb.String(), refs, nil
}

// isIdentChar returns true if c is a valid identifier character.
func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// sourceRefsMap converts a slice of SourceRef to a map keyed by placeholder.
func sourceRefsMap(refs []SourceRef) map[string]SourceRef {
	m := make(map[string]SourceRef, len(refs))
	for _, ref := range refs {
		m[ref.Placeholder] = ref
	}
	return m
}
