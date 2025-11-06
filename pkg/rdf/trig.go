package rdf

import (
	"fmt"
	"strconv"
	"strings"
)

// TriGParser parses TriG format (Turtle + named graphs)
type TriGParser struct {
	input            string
	pos              int
	length           int
	prefixes         map[string]string
	base             string
	blankNodeCounter int // Global blank node counter shared across all graph blocks
}

// NewTriGParser creates a new TriG parser
func NewTriGParser(input string) *TriGParser {
	return &TriGParser{
		input:    input,
		pos:      0,
		length:   len(input),
		prefixes: make(map[string]string),
	}
}

// SetBaseURI sets the base URI for resolving relative IRIs
func (p *TriGParser) SetBaseURI(baseURI string) {
	p.base = baseURI
}

// Parse parses the TriG document and returns quads
func (p *TriGParser) Parse() ([]*Quad, error) {
	var quads []*Quad

	for p.pos < p.length {
		p.skipWhitespaceAndComments()
		if p.pos >= p.length {
			break
		}

		// Check for PREFIX directive
		// @prefix must be lowercase (case-sensitive), PREFIX can be any case (case-insensitive)
		if p.matchExactKeyword("@prefix") || p.matchKeyword("PREFIX") {
			if err := p.parsePrefix(); err != nil {
				return nil, err
			}
			continue
		}

		// Check for BASE directive
		// @base must be lowercase (case-sensitive), BASE can be any case (case-insensitive)
		if p.matchExactKeyword("@base") || p.matchKeyword("BASE") {
			if err := p.parseBase(); err != nil {
				return nil, err
			}
			continue
		}

		// Check for GRAPH directive
		if p.matchKeyword("GRAPH") {
			graphQuads, err := p.parseGraphBlock()
			if err != nil {
				return nil, err
			}
			quads = append(quads, graphQuads...)
			continue
		}

		// Check for anonymous graph block: { triples }
		if p.input[p.pos] == '{' {
			graphQuads, err := p.parseAnonymousGraphBlock()
			if err != nil {
				return nil, err
			}
			quads = append(quads, graphQuads...)
			continue
		}

		// Check for named graph block: <iri> { triples } or _:bnode { triples }
		// Look ahead to see if there's a { after the first term
		savedPos := p.pos
		term, err := p.parseTerm()
		if err == nil && term != nil {
			p.skipWhitespaceAndComments()
			if p.pos < p.length && p.input[p.pos] == '{' {
				// It's a named graph block
				graphQuads, err := p.parseNamedGraphBlock(term)
				if err != nil {
					return nil, err
				}
				quads = append(quads, graphQuads...)
				continue
			}
		}
		// Not a graph block, restore position and parse as triple block using Turtle parser
		p.pos = savedPos

		// Parse triple block in default graph using Turtle parser
		triples, err := p.parseDefaultGraphTripleBlock()
		if err != nil {
			return nil, err
		}
		for _, triple := range triples {
			quad := NewQuad(triple.Subject, triple.Predicate, triple.Object, NewDefaultGraph())
			quads = append(quads, quad)
		}
	}

	return quads, nil
}

// parseGraphBlock parses a GRAPH block: GRAPH <iri> { triples }
func (p *TriGParser) parseGraphBlock() ([]*Quad, error) {
	p.skipWhitespaceAndComments()

	// Parse graph IRI
	graphTerm, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("expected graph IRI after GRAPH: %w", err)
	}

	// Graph must be a named node
	graphNode, ok := graphTerm.(*NamedNode)
	if !ok {
		return nil, fmt.Errorf("graph name must be an IRI, got: %T", graphTerm)
	}

	p.skipWhitespaceAndComments()

	// Expect '{'
	if p.pos >= p.length || p.input[p.pos] != '{' {
		return nil, fmt.Errorf("expected '{' after graph IRI")
	}
	p.pos++ // skip '{'

	// Find the matching closing brace
	braceStart := p.pos
	triples, newPos, err := p.parseTriplesBlock(braceStart)
	if err != nil {
		return nil, err
	}
	p.pos = newPos

	// Convert triples to quads with the graph name
	var quads []*Quad
	for _, triple := range triples {
		quads = append(quads, NewQuad(triple.Subject, triple.Predicate, triple.Object, graphNode))
	}

	return quads, nil
}

// parseAnonymousGraphBlock parses an anonymous graph block: { triples }
func (p *TriGParser) parseAnonymousGraphBlock() ([]*Quad, error) {
	// Expect '{'
	if p.pos >= p.length || p.input[p.pos] != '{' {
		return nil, fmt.Errorf("expected '{' at start of anonymous graph block")
	}
	p.pos++ // skip '{'

	// Use a blank node as the graph name
	graphNode := NewBlankNode(fmt.Sprintf("g%d", p.pos))

	// Find the matching closing brace
	braceStart := p.pos
	triples, newPos, err := p.parseTriplesBlock(braceStart)
	if err != nil {
		return nil, err
	}
	p.pos = newPos

	// Convert triples to quads with the graph name
	var quads []*Quad
	for _, triple := range triples {
		quads = append(quads, NewQuad(triple.Subject, triple.Predicate, triple.Object, graphNode))
	}

	return quads, nil
}

// parseNamedGraphBlock parses a named graph block: <iri> { triples } or _:bnode { triples }
func (p *TriGParser) parseNamedGraphBlock(graphTerm Term) ([]*Quad, error) {
	// graphTerm was already parsed by caller

	// Expect '{'
	if p.pos >= p.length || p.input[p.pos] != '{' {
		return nil, fmt.Errorf("expected '{' after graph name")
	}
	p.pos++ // skip '{'

	// Find the matching closing brace
	braceStart := p.pos
	triples, newPos, err := p.parseTriplesBlock(braceStart)
	if err != nil {
		return nil, err
	}
	p.pos = newPos

	// Convert triples to quads with the graph name
	var quads []*Quad
	for _, triple := range triples {
		quads = append(quads, NewQuad(triple.Subject, triple.Predicate, triple.Object, graphTerm))
	}

	return quads, nil
}

// parseTriplesBlock extracts and parses the triples content within a graph block using TurtleParser
// It finds the closing '}', extracts that content, delegates to TurtleParser, and returns the new position
func (p *TriGParser) parseTriplesBlock(startPos int) ([]*Triple, int, error) {
	// Find the matching closing brace
	braceCount := 1
	pos := startPos
	for pos < p.length && braceCount > 0 {
		if p.input[pos] == '{' {
			braceCount++
		} else if p.input[pos] == '}' {
			braceCount--
		}
		if braceCount > 0 {
			pos++
		}
	}

	if braceCount != 0 {
		return nil, pos, fmt.Errorf("unmatched braces in graph block")
	}

	// Extract the content between braces
	content := p.input[startPos:pos]

	// TriG allows optional trailing '.' in graph blocks, but Turtle parser expects it
	// So we need to ensure the content ends with '.' for each triple statement
	// The Turtle parser will handle the rest
	content = p.ensureProperTermination(content)

	// Create a TurtleParser for this content with inherited prefixes and base
	turtleParser := NewTurtleParser(content)
	turtleParser.prefixes = make(map[string]string)
	// Copy prefixes from TriG parser to Turtle parser
	for k, v := range p.prefixes {
		turtleParser.prefixes[k] = v
	}
	turtleParser.base = p.base
	turtleParser.blankNodeCounter = p.blankNodeCounter

	// Parse the triples
	triples, err := turtleParser.Parse()
	if err != nil {
		return nil, pos, fmt.Errorf("failed to parse triples in graph block: %w", err)
	}

	// Update TriG parser's blank node counter to maintain uniqueness across graph blocks
	p.blankNodeCounter = turtleParser.blankNodeCounter

	// Skip the closing '}'
	pos++

	return triples, pos, nil
}

// ensureProperTermination ensures content ends with '.' if it contains triples
// This is needed because TriG allows optional '.' but Turtle parser requires it
func (p *TriGParser) ensureProperTermination(content string) string {
	// Trim whitespace
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return content
	}

	// Find the last '.' that's not in a comment or string
	// Scan forward and track the last '.' we see outside comments
	lastDotPos := -1
	inComment := false
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(trimmed); i++ {
		ch := trimmed[i]

		// Handle escape sequences in strings
		if inString && ch == '\\' && i+1 < len(trimmed) {
			i++ // Skip next character
			continue
		}

		// Track string state
		if !inComment && (ch == '"' || ch == '\'') {
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				inString = false
				stringChar = 0
			}
			continue
		}

		// Track comment state (# starts a comment until end of line)
		if !inString && ch == '#' {
			inComment = true
			continue
		}

		// Newline ends a comment
		if ch == '\n' {
			inComment = false
			continue
		}

		// Record '.' positions that are not in comments or strings
		if !inComment && !inString && ch == '.' {
			lastDotPos = i
		}
	}

	// If we found a '.', we're good
	if lastDotPos >= 0 {
		return content
	}

	// No '.' found, add one
	return content + " ."
}

// parseDefaultGraphTripleBlock parses a triple block in the default graph using Turtle parser
func (p *TriGParser) parseDefaultGraphTripleBlock() ([]*Triple, error) {
	// Find the end of this triple block (marked by '.')
	// We need to extract content up to and including the '.'
	startPos := p.pos

	// Simple approach: find the next '.' that's not inside a string or IRI
	// For now, we'll scan to the next '.' considering quoted strings and IRIs
	endPos := p.findTripleBlockEnd(startPos)

	if endPos < 0 {
		return nil, fmt.Errorf("could not find end of triple block")
	}

	// Extract content including the '.'
	content := p.input[startPos:endPos]

	// Create a TurtleParser for this content with inherited prefixes and base
	turtleParser := NewTurtleParser(content)
	turtleParser.prefixes = make(map[string]string)
	// Copy prefixes from TriG parser to Turtle parser
	for k, v := range p.prefixes {
		turtleParser.prefixes[k] = v
	}
	turtleParser.base = p.base
	turtleParser.blankNodeCounter = p.blankNodeCounter

	// Parse the triples
	triples, err := turtleParser.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse triple block: %w", err)
	}

	// Update TriG parser's blank node counter to maintain uniqueness
	p.blankNodeCounter = turtleParser.blankNodeCounter

	// Update position to after the '.'
	p.pos = endPos

	return triples, nil
}

// findTripleBlockEnd finds the position after the '.' that ends a triple block
// It skips over strings, IRIs, and handles nesting
func (p *TriGParser) findTripleBlockEnd(start int) int {
	pos := start
	inString := false
	inIRI := false
	stringChar := byte(0)

	for pos < p.length {
		ch := p.input[pos]

		// Handle escape sequences
		if pos+1 < p.length && ch == '\\' {
			pos += 2 // Skip escaped character
			continue
		}

		// Track string state
		if !inIRI && (ch == '"' || ch == '\'') {
			if !inString {
				inString = true
				stringChar = ch
			} else if ch == stringChar {
				inString = false
				stringChar = 0
			}
		}

		// Track IRI state
		if !inString {
			if ch == '<' {
				inIRI = true
			} else if ch == '>' {
				inIRI = false
			}
		}

		// Check for triple block terminators when not in string or IRI
		if !inString && !inIRI {
			if ch == '.' {
				// Found the end, return position after the '.'
				return pos + 1
			}
			// Check if we hit a graph block or directive (means we went too far)
			if ch == '{' || ch == '}' {
				return -1
			}
		}

		pos++
	}

	return -1
}

// parseTerm parses an RDF term (IRI, blank node, or literal)
func (p *TriGParser) parseTerm() (Term, error) {
	p.skipWhitespaceAndComments()
	if p.pos >= p.length {
		return nil, fmt.Errorf("unexpected end of input")
	}

	ch := p.input[p.pos]

	switch ch {
	case '<':
		return p.parseIRI()
	case '_':
		return p.parseBlankNode()
	case '"':
		return p.parseLiteral()
	case ':':
		// Prefixed name with empty prefix: :localName
		return p.parsePrefixedName()
	case '?', '$':
		// Variables not supported in data (only in queries)
		return nil, fmt.Errorf("variables not allowed in data")
	default:
		// Number literal
		if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
			return p.parseNumber()
		}

		// Check for boolean literals (case-sensitive per Turtle spec)
		if p.matchExactKeyword("true") {
			return NewBooleanLiteral(true), nil
		}
		if p.matchExactKeyword("false") {
			return NewBooleanLiteral(false), nil
		}

		// Check for 'a' keyword (shorthand for rdf:type)
		if ch == 'a' {
			// Check if next character is a name character
			if p.pos+1 < p.length {
				next := p.input[p.pos+1]
				isName := (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || (next >= '0' && next <= '9') || next == '_' || next == '-'
				if !isName {
					p.pos++ // skip 'a'
					return NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), nil
				}
			} else {
				// 'a' at end of input
				p.pos++ // skip 'a'
				return NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), nil
			}
		}

		// Try prefixed name
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			return p.parsePrefixedName()
		}
		return nil, fmt.Errorf("unexpected character: %c", ch)
	}
}

// parseIRI parses an IRI: <http://example.org/resource>
func (p *TriGParser) parseIRI() (*NamedNode, error) {
	if p.pos >= p.length || p.input[p.pos] != '<' {
		return nil, fmt.Errorf("expected '<'")
	}
	p.pos++ // skip '<'

	start := p.pos
	for p.pos < p.length && p.input[p.pos] != '>' {
		p.pos++
	}

	if p.pos >= p.length {
		return nil, fmt.Errorf("unterminated IRI")
	}

	iri := p.input[start:p.pos]
	p.pos++ // skip '>'

	// Resolve against base if relative
	if p.base != "" && !strings.Contains(iri, "://") {
		iri = p.base + iri
	}

	return NewNamedNode(iri), nil
}

// parseBlankNode parses a blank node: _:b1
func (p *TriGParser) parseBlankNode() (*BlankNode, error) {
	if p.pos+2 > p.length || p.input[p.pos:p.pos+2] != "_:" {
		return nil, fmt.Errorf("expected '_:'")
	}
	p.pos += 2 // skip '_:'

	start := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			break
		}
		p.pos++
	}

	if p.pos == start {
		return nil, fmt.Errorf("blank node ID required after '_:'")
	}

	id := p.input[start:p.pos]
	return NewBlankNode(id), nil
}

// parseLiteral parses a literal: "value" or "value"@lang or "value"^^<type>
func (p *TriGParser) parseLiteral() (*Literal, error) {
	if p.pos >= p.length || p.input[p.pos] != '"' {
		return nil, fmt.Errorf("expected '\"'")
	}
	p.pos++ // skip '"'

	var value strings.Builder
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == '"' {
			p.pos++ // skip closing '"'
			break
		}
		if ch == '\\' && p.pos+1 < p.length {
			// Handle escape sequences
			p.pos++
			next := p.input[p.pos]
			switch next {
			case 'n':
				value.WriteByte('\n')
			case 't':
				value.WriteByte('\t')
			case 'r':
				value.WriteByte('\r')
			case '"':
				value.WriteByte('"')
			case '\\':
				value.WriteByte('\\')
			default:
				value.WriteByte(next)
			}
			p.pos++
			continue
		}
		value.WriteByte(ch)
		p.pos++
	}

	lit := &Literal{Value: value.String()}

	// Check for language tag or datatype
	if p.pos < p.length {
		if p.input[p.pos] == '@' {
			// Language tag
			p.pos++ // skip '@'
			start := p.pos
			for p.pos < p.length {
				ch := p.input[p.pos]
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '-') {
					break
				}
				p.pos++
			}
			lit.Language = p.input[start:p.pos]
		} else if p.pos+2 < p.length && p.input[p.pos:p.pos+2] == "^^" {
			// Datatype
			p.pos += 2 // skip '^^'
			datatype, err := p.parseTerm()
			if err != nil {
				return nil, fmt.Errorf("failed to parse datatype: %w", err)
			}
			if dt, ok := datatype.(*NamedNode); ok {
				lit.Datatype = dt
			} else {
				return nil, fmt.Errorf("datatype must be an IRI")
			}
		}
	}

	return lit, nil
}

// parseNumber parses a number literal (integer, decimal, or double)
func (p *TriGParser) parseNumber() (Term, error) {
	start := p.pos
	isDecimal := false
	isDouble := false

	// Handle sign
	if p.pos < p.length && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		p.pos++
	}

	// Read integer part digits
	hasDigits := false
	for p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
		hasDigits = true
	}

	if !hasDigits {
		return nil, fmt.Errorf("expected digits in number")
	}

	// Check for decimal point
	if p.pos < p.length && p.input[p.pos] == '.' {
		// Look ahead to check if this is really a decimal or end of statement
		if p.pos+1 < p.length {
			nextCh := p.input[p.pos+1]
			// If next char is a digit, it's a decimal
			if nextCh >= '0' && nextCh <= '9' {
				isDecimal = true
				p.pos++ // skip '.'
				// Read fractional digits
				for p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
					p.pos++
				}
			}
		}
	}

	// Check for exponent (e or E) which makes it a double
	if p.pos < p.length && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		isDouble = true
		p.pos++ // skip 'e' or 'E'

		// Optional sign after exponent
		if p.pos < p.length && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}

		// Read exponent digits
		expHasDigits := false
		for p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
			expHasDigits = true
		}

		if !expHasDigits {
			return nil, fmt.Errorf("expected digits in exponent")
		}
	}

	numStr := p.input[start:p.pos]

	// Return appropriate type preserving original lexical form
	if isDouble {
		// Validate it's a valid double
		_, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse double: %w", err)
		}
		// Preserve original lexical form
		return NewLiteralWithDatatype(numStr, XSDDouble), nil
	} else if isDecimal {
		// Validate it's a valid decimal
		_, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse decimal: %w", err)
		}
		// Preserve original lexical form
		return NewLiteralWithDatatype(numStr, XSDDecimal), nil
	} else {
		// Integer - validate it's a valid integer
		_, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse integer: %w", err)
		}
		// Preserve original lexical form
		return NewLiteralWithDatatype(numStr, XSDInteger), nil
	}
}

// parsePrefixedName parses a prefixed name: prefix:localName or :localName (empty prefix)
func (p *TriGParser) parsePrefixedName() (*NamedNode, error) {
	start := p.pos

	// Parse prefix part (may be empty for :localName)
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == ':' {
			break
		}
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			break
		}
		p.pos++
	}

	if p.pos >= p.length || p.input[p.pos] != ':' {
		return nil, fmt.Errorf("expected ':' in prefixed name")
	}

	prefix := p.input[start:p.pos]
	p.pos++ // skip ':'

	// Parse local name part
	localStart := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.') {
			break
		}
		p.pos++
	}

	localName := p.input[localStart:p.pos]

	// Resolve prefix (empty string is a valid prefix)
	namespace, ok := p.prefixes[prefix]
	if !ok {
		return nil, fmt.Errorf("undefined prefix: %s", prefix)
	}

	return NewNamedNode(namespace + localName), nil
}

// parsePrefix parses a PREFIX directive: PREFIX prefix: <iri>
func (p *TriGParser) parsePrefix() error {
	p.skipWhitespaceAndComments()

	// Parse prefix name
	start := p.pos
	for p.pos < p.length && p.input[p.pos] != ':' {
		p.pos++
	}
	if p.pos >= p.length {
		return fmt.Errorf("expected ':' in PREFIX")
	}

	prefix := strings.TrimSpace(p.input[start:p.pos])
	p.pos++ // skip ':'

	p.skipWhitespaceAndComments()

	// Parse IRI
	iri, err := p.parseIRI()
	if err != nil {
		return fmt.Errorf("failed to parse prefix IRI: %w", err)
	}

	p.prefixes[prefix] = iri.IRI

	// Skip optional '.'
	p.skipWhitespaceAndComments()
	if p.pos < p.length && p.input[p.pos] == '.' {
		p.pos++
	}

	return nil
}

// parseBase parses a BASE directive: BASE <iri>
func (p *TriGParser) parseBase() error {
	p.skipWhitespaceAndComments()

	// Parse IRI
	iri, err := p.parseIRI()
	if err != nil {
		return fmt.Errorf("failed to parse base IRI: %w", err)
	}

	p.base = iri.IRI

	// Skip optional '.'
	p.skipWhitespaceAndComments()
	if p.pos < p.length && p.input[p.pos] == '.' {
		p.pos++
	}

	return nil
}

// skipWhitespaceAndComments skips whitespace and comments
func (p *TriGParser) skipWhitespaceAndComments() {
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
			continue
		}
		if ch == '#' {
			// Skip comment until end of line
			for p.pos < p.length && p.input[p.pos] != '\n' {
				p.pos++
			}
			continue
		}
		break
	}
}

// matchKeyword checks if the current position matches a keyword (case-insensitive)
func (p *TriGParser) matchKeyword(keyword string) bool {
	if p.pos+len(keyword) > p.length {
		return false
	}

	// Check if keyword matches
	if !strings.EqualFold(p.input[p.pos:p.pos+len(keyword)], keyword) {
		return false
	}

	// Check that keyword is followed by whitespace or special char
	if p.pos+len(keyword) < p.length {
		nextCh := p.input[p.pos+len(keyword)]
		if (nextCh >= 'a' && nextCh <= 'z') || (nextCh >= 'A' && nextCh <= 'Z') || (nextCh >= '0' && nextCh <= '9') {
			return false
		}
	}

	p.pos += len(keyword)
	return true
}

// matchExactKeyword checks if the current position matches a keyword (case-sensitive)
func (p *TriGParser) matchExactKeyword(keyword string) bool {
	if p.pos+len(keyword) > p.length {
		return false
	}

	// Check if keyword matches exactly (case-sensitive)
	if p.input[p.pos:p.pos+len(keyword)] != keyword {
		return false
	}

	// Check that keyword is followed by whitespace or special char
	if p.pos+len(keyword) < p.length {
		nextCh := p.input[p.pos+len(keyword)]
		if (nextCh >= 'a' && nextCh <= 'z') || (nextCh >= 'A' && nextCh <= 'Z') || (nextCh >= '0' && nextCh <= '9') {
			return false
		}
	}

	p.pos += len(keyword)
	return true
}
