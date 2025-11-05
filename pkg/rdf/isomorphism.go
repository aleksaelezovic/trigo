package rdf

import (
	"fmt"
	"sort"
)

// AreGraphsIsomorphic checks if two sets of triples are isomorphic,
// accounting for blank node label differences.
// Two graphs are isomorphic if there exists a bijection between their
// blank nodes such that when applied, the graphs are identical.
func AreGraphsIsomorphic(expected, actual []*Triple) bool {
	// Quick check: same number of triples
	if len(expected) != len(actual) {
		return false
	}

	// Extract blank nodes from both graphs
	expectedBlanks := extractBlankNodeLabels(expected)
	actualBlanks := extractBlankNodeLabels(actual)

	// Quick check: same number of blank nodes
	if len(expectedBlanks) != len(actualBlanks) {
		return false
	}

	// If no blank nodes, use simple comparison
	if len(expectedBlanks) == 0 {
		return simpleCompare(expected, actual)
	}

	// Sort blank nodes by degree (optimization: match high-degree nodes first)
	expectedBlanks = sortByDegree(expectedBlanks, expected)
	actualBlanks = sortByDegree(actualBlanks, actual)

	// Find isomorphic mapping via backtracking
	mapping := make(map[string]string)
	usedTargets := make(map[string]bool)
	return backtrack(expected, actual, expectedBlanks, actualBlanks, mapping, usedTargets, 0)
}

// extractBlankNodeLabels extracts all unique blank node labels from a set of triples
func extractBlankNodeLabels(triples []*Triple) []string {
	blanks := make(map[string]bool)
	for _, triple := range triples {
		if bn, ok := triple.Subject.(*BlankNode); ok {
			blanks[bn.ID] = true
		}
		if bn, ok := triple.Object.(*BlankNode); ok {
			blanks[bn.ID] = true
		}
	}

	// Convert to sorted slice for deterministic ordering
	result := make([]string, 0, len(blanks))
	for label := range blanks {
		result = append(result, label)
	}
	sort.Strings(result)
	return result
}

// sortByDegree sorts blank nodes by their degree (number of triples they appear in)
// This optimization helps backtracking by trying to match highly-connected nodes first
func sortByDegree(blanks []string, triples []*Triple) []string {
	degrees := make(map[string]int)
	for _, blank := range blanks {
		degrees[blank] = 0
	}

	for _, triple := range triples {
		if bn, ok := triple.Subject.(*BlankNode); ok {
			degrees[bn.ID]++
		}
		if bn, ok := triple.Object.(*BlankNode); ok {
			degrees[bn.ID]++
		}
	}

	// Sort by degree (descending)
	sort.Slice(blanks, func(i, j int) bool {
		return degrees[blanks[i]] > degrees[blanks[j]]
	})

	return blanks
}

// simpleCompare compares two triple sets without considering blank node isomorphism
func simpleCompare(expected, actual []*Triple) bool {
	expectedMap := make(map[string]bool)
	for _, triple := range expected {
		key := tripleKey(triple, nil)
		expectedMap[key] = true
	}

	for _, triple := range actual {
		key := tripleKey(triple, nil)
		if !expectedMap[key] {
			return false
		}
	}

	return true
}

// backtrack recursively tries to find a valid mapping between blank nodes
func backtrack(expected, actual []*Triple, expectedBlanks, actualBlanks []string,
	mapping map[string]string, usedTargets map[string]bool, index int) bool {

	// Base case: all blank nodes have been mapped
	if index == len(expectedBlanks) {
		return verifyMapping(expected, actual, mapping)
	}

	currentBlank := expectedBlanks[index]

	// Try mapping current blank node to each candidate
	for _, candidateBlank := range actualBlanks {
		// Skip if this target blank node is already mapped
		if usedTargets[candidateBlank] {
			continue
		}

		// Try this mapping
		mapping[currentBlank] = candidateBlank
		usedTargets[candidateBlank] = true

		// Early pruning: check if mapping is still consistent
		if isConsistentSoFar(expected, actual, mapping) {
			if backtrack(expected, actual, expectedBlanks, actualBlanks, mapping, usedTargets, index+1) {
				return true
			}
		}

		// Backtrack
		delete(mapping, currentBlank)
		delete(usedTargets, candidateBlank)
	}

	return false
}

// isConsistentSoFar checks if the current partial mapping is consistent
// This is an optimization to prune the search space early
func isConsistentSoFar(expected, actual []*Triple, mapping map[string]string) bool {
	// For each triple in expected that only contains mapped blank nodes,
	// check if there's a corresponding triple in actual
	for _, triple := range expected {
		// Check if all blank nodes in this triple are mapped
		subjectMapped := true
		objectMapped := true

		if bn, ok := triple.Subject.(*BlankNode); ok {
			if _, exists := mapping[bn.ID]; !exists {
				subjectMapped = false
			}
		}

		if bn, ok := triple.Object.(*BlankNode); ok {
			if _, exists := mapping[bn.ID]; !exists {
				objectMapped = false
			}
		}

		// If all blank nodes in this triple are mapped, verify it exists in actual
		if subjectMapped && objectMapped {
			found := false
			mappedKey := tripleKey(triple, mapping)

			for _, actualTriple := range actual {
				if tripleKey(actualTriple, nil) == mappedKey {
					found = true
					break
				}
			}

			if !found {
				return false
			}
		}
	}

	return true
}

// verifyMapping checks if the given mapping makes the graphs identical
func verifyMapping(expected, actual []*Triple, mapping map[string]string) bool {
	// Create a set of mapped expected triples
	expectedMapped := make(map[string]bool)
	for _, triple := range expected {
		key := tripleKey(triple, mapping)
		expectedMapped[key] = true
	}

	// Create a set of actual triples
	actualSet := make(map[string]bool)
	for _, triple := range actual {
		key := tripleKey(triple, nil)
		actualSet[key] = true
	}

	// Check if they're identical
	if len(expectedMapped) != len(actualSet) {
		return false
	}

	for key := range expectedMapped {
		if !actualSet[key] {
			return false
		}
	}

	return true
}

// tripleKey creates a string key for a triple, applying blank node mapping if provided
func tripleKey(triple *Triple, mapping map[string]string) string {
	subject := termString(triple.Subject, mapping)
	predicate := termString(triple.Predicate, mapping)
	object := termString(triple.Object, mapping)
	return fmt.Sprintf("%s|%s|%s", subject, predicate, object)
}

// termString converts a term to string, applying blank node mapping if applicable
func termString(term Term, mapping map[string]string) string {
	if mapping != nil {
		if bn, ok := term.(*BlankNode); ok {
			if mapped, exists := mapping[bn.ID]; exists {
				return "_:" + mapped
			}
		}
	}
	return term.String()
}

// AreQuadsIsomorphic checks if two sets of quads are isomorphic,
// accounting for blank node label differences in both triples and graph names.
func AreQuadsIsomorphic(expected, actual []*Quad) bool {
	// Quick check: same number of quads
	if len(expected) != len(actual) {
		return false
	}

	// Extract blank nodes from both graphs (including graph names)
	expectedBlanks := extractBlankNodeLabelsFromQuads(expected)
	actualBlanks := extractBlankNodeLabelsFromQuads(actual)

	// Quick check: same number of blank nodes
	if len(expectedBlanks) != len(actualBlanks) {
		return false
	}

	// If no blank nodes, use simple comparison
	if len(expectedBlanks) == 0 {
		return simpleCompareQuads(expected, actual)
	}

	// Sort blank nodes by degree
	expectedBlanks = sortByDegreeQuads(expectedBlanks, expected)
	actualBlanks = sortByDegreeQuads(actualBlanks, actual)

	// Find isomorphic mapping via backtracking
	mapping := make(map[string]string)
	usedTargets := make(map[string]bool)
	return backtrackQuads(expected, actual, expectedBlanks, actualBlanks, mapping, usedTargets, 0)
}

// extractBlankNodeLabelsFromQuads extracts all unique blank node labels from quads
func extractBlankNodeLabelsFromQuads(quads []*Quad) []string {
	blanks := make(map[string]bool)
	for _, quad := range quads {
		if bn, ok := quad.Subject.(*BlankNode); ok {
			blanks[bn.ID] = true
		}
		if bn, ok := quad.Object.(*BlankNode); ok {
			blanks[bn.ID] = true
		}
		if bn, ok := quad.Graph.(*BlankNode); ok {
			blanks[bn.ID] = true
		}
	}

	result := make([]string, 0, len(blanks))
	for label := range blanks {
		result = append(result, label)
	}
	sort.Strings(result)
	return result
}

// sortByDegreeQuads sorts blank nodes by their degree in quads
func sortByDegreeQuads(blanks []string, quads []*Quad) []string {
	degrees := make(map[string]int)
	for _, blank := range blanks {
		degrees[blank] = 0
	}

	for _, quad := range quads {
		if bn, ok := quad.Subject.(*BlankNode); ok {
			degrees[bn.ID]++
		}
		if bn, ok := quad.Object.(*BlankNode); ok {
			degrees[bn.ID]++
		}
		if bn, ok := quad.Graph.(*BlankNode); ok {
			degrees[bn.ID]++
		}
	}

	sort.Slice(blanks, func(i, j int) bool {
		return degrees[blanks[i]] > degrees[blanks[j]]
	})

	return blanks
}

// simpleCompareQuads compares two quad sets without considering blank node isomorphism
func simpleCompareQuads(expected, actual []*Quad) bool {
	expectedMap := make(map[string]bool)
	for _, quad := range expected {
		key := quadKey(quad, nil)
		expectedMap[key] = true
	}

	for _, quad := range actual {
		key := quadKey(quad, nil)
		if !expectedMap[key] {
			return false
		}
	}

	return true
}

// backtrackQuads recursively tries to find a valid mapping between blank nodes in quads
func backtrackQuads(expected, actual []*Quad, expectedBlanks, actualBlanks []string,
	mapping map[string]string, usedTargets map[string]bool, index int) bool {

	if index == len(expectedBlanks) {
		return verifyMappingQuads(expected, actual, mapping)
	}

	currentBlank := expectedBlanks[index]

	for _, candidateBlank := range actualBlanks {
		if usedTargets[candidateBlank] {
			continue
		}

		mapping[currentBlank] = candidateBlank
		usedTargets[candidateBlank] = true

		if isConsistentSoFarQuads(expected, actual, mapping) {
			if backtrackQuads(expected, actual, expectedBlanks, actualBlanks, mapping, usedTargets, index+1) {
				return true
			}
		}

		delete(mapping, currentBlank)
		delete(usedTargets, candidateBlank)
	}

	return false
}

// isConsistentSoFarQuads checks if the current partial mapping is consistent for quads
func isConsistentSoFarQuads(expected, actual []*Quad, mapping map[string]string) bool {
	for _, quad := range expected {
		subjectMapped := true
		objectMapped := true
		graphMapped := true

		if bn, ok := quad.Subject.(*BlankNode); ok {
			if _, exists := mapping[bn.ID]; !exists {
				subjectMapped = false
			}
		}

		if bn, ok := quad.Object.(*BlankNode); ok {
			if _, exists := mapping[bn.ID]; !exists {
				objectMapped = false
			}
		}

		if bn, ok := quad.Graph.(*BlankNode); ok {
			if _, exists := mapping[bn.ID]; !exists {
				graphMapped = false
			}
		}

		if subjectMapped && objectMapped && graphMapped {
			found := false
			mappedKey := quadKey(quad, mapping)

			for _, actualQuad := range actual {
				if quadKey(actualQuad, nil) == mappedKey {
					found = true
					break
				}
			}

			if !found {
				return false
			}
		}
	}

	return true
}

// verifyMappingQuads checks if the given mapping makes the quad graphs identical
func verifyMappingQuads(expected, actual []*Quad, mapping map[string]string) bool {
	expectedMapped := make(map[string]bool)
	for _, quad := range expected {
		key := quadKey(quad, mapping)
		expectedMapped[key] = true
	}

	actualSet := make(map[string]bool)
	for _, quad := range actual {
		key := quadKey(quad, nil)
		actualSet[key] = true
	}

	if len(expectedMapped) != len(actualSet) {
		return false
	}

	for key := range expectedMapped {
		if !actualSet[key] {
			return false
		}
	}

	return true
}

// quadKey creates a string key for a quad, applying blank node mapping if provided
func quadKey(quad *Quad, mapping map[string]string) string {
	subject := termString(quad.Subject, mapping)
	predicate := termString(quad.Predicate, mapping)
	object := termString(quad.Object, mapping)
	graph := termString(quad.Graph, mapping)
	return fmt.Sprintf("%s|%s|%s|%s", subject, predicate, object, graph)
}
