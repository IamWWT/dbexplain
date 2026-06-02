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

// preprocess scans DSL input for @label.table patterns and replaces them
// with unique placeholders (__dsl_N__). The placeholders are valid SQL
// identifiers that the sqlast parser can handle as table names.
//
// The returned refs slice maps each placeholder to its original SourceRef.
func preprocess(input string) (string, []SourceRef, error) {
	var refs []SourceRef
	counter := 0
	seen := make(map[string]string)

	result := dslRefPattern.ReplaceAllStringFunc(input, func(match string) string {
		// match is "@label.table" — strip "@" and split on "."
		inner := match[1:] // skip '@'
		label, table, _ := strings.Cut(inner, ".")
		if label == "" || table == "" {
			return match
		}

		// Skip if no label (should not happen with the regex)
		if label == "" {
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

	// Build a reverse lookup: placeholder → SourceRef
	refMap := make(map[string]SourceRef, len(refs))
	for _, ref := range refs {
		refMap[ref.Placeholder] = ref
	}

	return result, refs, nil
}

// sourceRefsMap converts a slice of SourceRef to a map keyed by placeholder.
func sourceRefsMap(refs []SourceRef) map[string]SourceRef {
	m := make(map[string]SourceRef, len(refs))
	for _, ref := range refs {
		m[ref.Placeholder] = ref
	}
	return m
}
