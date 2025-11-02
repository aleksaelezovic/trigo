package rdf

import (
	"fmt"
	"strconv"
	"strings"
)

// TriGParser parses TriG format (Turtle + named graphs)
type TriGParser struct {
	input    string
	pos      int
	length   int
	prefixes map[string]string
	base     string
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

// Parse parses the TriG document and returns quads
func (p *TriGParser) Parse() ([]*Quad, error) {
	var quads []*Quad

	for p.pos < p.length {
		p.skipWhitespaceAndComments()
		if p.pos >= p.length {
			break
		}

		// Check for PREFIX directive
		if p.matchKeyword("@prefix") || p.matchKeyword("PREFIX") {
			if err := p.parsePrefix(); err != nil {
				return nil, err
			}
			continue
		}

		// Check for BASE directive
		if p.matchKeyword("@base") || p.matchKeyword("BASE") {
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
		// Not a graph block, restore position and parse as triple
		p.pos = savedPos

		// Parse triple in default graph
		triple, err := p.parseTriple()
		if err != nil {
			return nil, err
		}
		if triple != nil {
			quad := NewQuad(triple.Subject, triple.Predicate, triple.Object, NewDefaultGraph())
			quads = append(quads, quad)
		}

		// Skip optional '.'
		p.skipWhitespaceAndComments()
		if p.pos < p.length && p.input[p.pos] == '.' {
			p.pos++
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

	// Parse triples until '}'
	var quads []*Quad
	for {
		p.skipWhitespaceAndComments()
		if p.pos >= p.length {
			return nil, fmt.Errorf("unexpected end of input, expected '}'")
		}

		// Check for closing '}'
		if p.input[p.pos] == '}' {
			p.pos++ // skip '}'
			break
		}

		// Parse triple
		triple, err := p.parseTriple()
		if err != nil {
			return nil, err
		}
		if triple != nil {
			quad := NewQuad(triple.Subject, triple.Predicate, triple.Object, graphNode)
			quads = append(quads, quad)
		}

		// Skip optional '.'
		p.skipWhitespaceAndComments()
		if p.pos < p.length && p.input[p.pos] == '.' {
			p.pos++
		}
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

	// Parse triples until '}'
	var quads []*Quad
	for {
		p.skipWhitespaceAndComments()
		if p.pos >= p.length {
			return nil, fmt.Errorf("unexpected end of input, expected '}'")
		}

		// Check for closing '}'
		if p.input[p.pos] == '}' {
			p.pos++ // skip '}'
			break
		}

		// Parse triple
		triple, err := p.parseTriple()
		if err != nil {
			return nil, err
		}
		if triple != nil {
			quad := NewQuad(triple.Subject, triple.Predicate, triple.Object, graphNode)
			quads = append(quads, quad)
		}

		// Skip optional '.'
		p.skipWhitespaceAndComments()
		if p.pos < p.length && p.input[p.pos] == '.' {
			p.pos++
		}
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

	// Parse triples until '}'
	var quads []*Quad
	for {
		p.skipWhitespaceAndComments()
		if p.pos >= p.length {
			return nil, fmt.Errorf("unexpected end of input, expected '}'")
		}

		// Check for closing '}'
		if p.input[p.pos] == '}' {
			p.pos++ // skip '}'
			break
		}

		// Parse triple
		triple, err := p.parseTriple()
		if err != nil {
			return nil, err
		}
		if triple != nil {
			quad := NewQuad(triple.Subject, triple.Predicate, triple.Object, graphTerm)
			quads = append(quads, quad)
		}

		// Skip optional '.'
		p.skipWhitespaceAndComments()
		if p.pos < p.length && p.input[p.pos] == '.' {
			p.pos++
		}
	}

	return quads, nil
}

// parseTriple parses a single triple (reusing Turtle parser logic)
func (p *TriGParser) parseTriple() (*Triple, error) {
	p.skipWhitespaceAndComments()

	// Parse subject
	subject, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject: %w", err)
	}

	p.skipWhitespaceAndComments()

	// Parse predicate
	predicate, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("failed to parse predicate: %w", err)
	}

	p.skipWhitespaceAndComments()

	// Parse object
	object, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("failed to parse object: %w", err)
	}

	return NewTriple(subject, predicate, object), nil
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

// parseNumber parses a number literal (integer or double)
func (p *TriGParser) parseNumber() (Term, error) {
	start := p.pos

	// Handle sign
	if p.pos < p.length && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		p.pos++
	}

	// Read digits
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
		p.pos++
		// Read fractional digits
		for p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}

		// It's a double
		numStr := p.input[start:p.pos]
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse double: %w", err)
		}
		return NewDoubleLiteral(val), nil
	}

	// It's an integer
	numStr := p.input[start:p.pos]
	val, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse integer: %w", err)
	}
	return NewIntegerLiteral(val), nil
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

// matchKeyword checks if the current position matches a keyword
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
