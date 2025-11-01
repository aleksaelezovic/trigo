package rdf

import (
	"fmt"
	"strconv"
	"strings"
)

// TurtleParser is a simple Turtle/N-Triples parser for loading test data
type TurtleParser struct {
	input    string
	pos      int
	length   int
	prefixes map[string]string
}

// NewTurtleParser creates a new Turtle parser
func NewTurtleParser(input string) *TurtleParser {
	return &TurtleParser{
		input:    input,
		pos:      0,
		length:   len(input),
		prefixes: make(map[string]string),
	}
}

// Parse parses the Turtle document and returns triples
func (p *TurtleParser) Parse() ([]*Triple, error) {
	var triples []*Triple

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

		// Parse triple block (may return multiple triples due to property list syntax)
		blockTriples, err := p.parseTripleBlock()
		if err != nil {
			return nil, err
		}
		triples = append(triples, blockTriples...)
	}

	return triples, nil
}

// skipWhitespaceAndComments skips whitespace and comments
func (p *TurtleParser) skipWhitespaceAndComments() {
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
func (p *TurtleParser) matchKeyword(keyword string) bool {
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
		if !((nextCh >= 'a' && nextCh <= 'z') || (nextCh >= 'A' && nextCh <= 'Z') || (nextCh >= '0' && nextCh <= '9')) {
			p.pos += len(keyword)
			return true
		}
	} else {
		p.pos += len(keyword)
		return true
	}

	return false
}

// parsePrefix parses a PREFIX declaration
func (p *TurtleParser) parsePrefix() error {
	p.skipWhitespaceAndComments()

	// Read prefix name (until ':')
	prefixStart := p.pos
	for p.pos < p.length && p.input[p.pos] != ':' {
		p.pos++
	}
	prefix := p.input[prefixStart:p.pos]

	if p.pos >= p.length || p.input[p.pos] != ':' {
		return fmt.Errorf("expected ':' after prefix name")
	}
	p.pos++ // skip ':'

	p.skipWhitespaceAndComments()

	// Read IRI
	iri, err := p.parseIRI()
	if err != nil {
		return fmt.Errorf("failed to parse prefix IRI: %w", err)
	}

	p.prefixes[prefix] = iri

	p.skipWhitespaceAndComments()
	if p.pos < p.length && (p.input[p.pos] == '.' || p.input[p.pos] == ';') {
		p.pos++ // skip ending
	}

	return nil
}

// parseBase parses a BASE declaration
func (p *TurtleParser) parseBase() error {
	p.skipWhitespaceAndComments()

	// Read IRI
	_, err := p.parseIRI()
	if err != nil {
		return fmt.Errorf("failed to parse base IRI: %w", err)
	}

	p.skipWhitespaceAndComments()
	if p.pos < p.length && (p.input[p.pos] == '.' || p.input[p.pos] == ';') {
		p.pos++ // skip ending
	}

	return nil
}

// parseTripleBlock parses a block of triples with property list syntax
func (p *TurtleParser) parseTripleBlock() ([]*Triple, error) {
	var triples []*Triple

	// Parse subject
	subject, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject: %w", err)
	}

	// Parse predicate-object pairs
	for {
		p.skipWhitespaceAndComments()

		// Parse predicate
		predicate, err := p.parseTerm()
		if err != nil {
			return nil, fmt.Errorf("failed to parse predicate: %w", err)
		}

		// Parse objects (can be multiple with comma separator)
		for {
			p.skipWhitespaceAndComments()

			// Parse object
			object, err := p.parseTerm()
			if err != nil {
				return nil, fmt.Errorf("failed to parse object: %w", err)
			}

			triples = append(triples, NewTriple(subject, predicate, object))

			p.skipWhitespaceAndComments()

			// Check for comma (more objects with same predicate)
			if p.pos < p.length && p.input[p.pos] == ',' {
				p.pos++ // skip ','
				continue
			}
			break
		}

		p.skipWhitespaceAndComments()

		// Check for semicolon (more predicates with same subject)
		if p.pos < p.length && p.input[p.pos] == ';' {
			p.pos++ // skip ';'
			p.skipWhitespaceAndComments()
			// Check if there's actually a predicate following (not just a trailing semicolon)
			if p.pos < p.length && p.input[p.pos] != '.' {
				continue
			}
		}

		break
	}

	p.skipWhitespaceAndComments()

	// Expect '.'
	if p.pos >= p.length || p.input[p.pos] != '.' {
		return nil, fmt.Errorf("expected '.' at end of triple")
	}
	p.pos++ // skip '.'

	return triples, nil
}

// parseTerm parses an RDF term (IRI, blank node, or literal)
func (p *TurtleParser) parseTerm() (Term, error) {
	p.skipWhitespaceAndComments()

	if p.pos >= p.length {
		return nil, fmt.Errorf("unexpected end of input")
	}

	ch := p.input[p.pos]

	// IRI in angle brackets
	if ch == '<' {
		iri, err := p.parseIRI()
		if err != nil {
			return nil, err
		}
		return NewNamedNode(iri), nil
	}

	// Blank node
	if ch == '_' && p.pos+1 < p.length && p.input[p.pos+1] == ':' {
		return p.parseBlankNode()
	}

	// String literal
	if ch == '"' {
		return p.parseLiteral()
	}

	// Number literal
	if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
		return p.parseNumber()
	}

	// Check for 'a' keyword (shorthand for rdf:type)
	if ch == 'a' && (p.pos+1 >= p.length || !isNameChar(p.input[p.pos+1])) {
		p.pos++ // skip 'a'
		return NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), nil
	}

	// Prefixed name
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == ':' {
		return p.parsePrefixedName()
	}

	return nil, fmt.Errorf("unexpected character: %c at position %d", ch, p.pos)
}

// isNameChar checks if a character can be part of a name
func isNameChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-'
}

// parseIRI parses an IRI in angle brackets
func (p *TurtleParser) parseIRI() (string, error) {
	if p.pos >= p.length || p.input[p.pos] != '<' {
		return "", fmt.Errorf("expected '<' at start of IRI")
	}
	p.pos++ // skip '<'

	start := p.pos
	for p.pos < p.length && p.input[p.pos] != '>' {
		p.pos++
	}

	if p.pos >= p.length {
		return "", fmt.Errorf("unclosed IRI")
	}

	iri := p.input[start:p.pos]
	p.pos++ // skip '>'

	return iri, nil
}

// parseBlankNode parses a blank node
func (p *TurtleParser) parseBlankNode() (Term, error) {
	if p.pos+1 >= p.length || p.input[p.pos] != '_' || p.input[p.pos+1] != ':' {
		return nil, fmt.Errorf("expected '_:' at start of blank node")
	}
	p.pos += 2 // skip '_:'

	start := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			break
		}
		p.pos++
	}

	label := p.input[start:p.pos]
	return NewBlankNode(label), nil
}

// parseLiteral parses a string literal
func (p *TurtleParser) parseLiteral() (Term, error) {
	if p.pos >= p.length || p.input[p.pos] != '"' {
		return nil, fmt.Errorf("expected '\"' at start of literal")
	}
	p.pos++ // skip '"'

	var value strings.Builder
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == '"' {
			break
		}
		if ch == '\\' && p.pos+1 < p.length {
			// Handle escape sequences
			p.pos++
			switch p.input[p.pos] {
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
				value.WriteByte(p.input[p.pos])
			}
			p.pos++
		} else {
			value.WriteByte(ch)
			p.pos++
		}
	}

	if p.pos >= p.length {
		return nil, fmt.Errorf("unclosed string literal")
	}
	p.pos++ // skip closing '"'

	// Check for language tag or datatype
	p.skipWhitespaceAndComments()
	if p.pos < p.length && p.input[p.pos] == '@' {
		// Language tag
		p.pos++ // skip '@'
		langStart := p.pos
		for p.pos < p.length && ((p.input[p.pos] >= 'a' && p.input[p.pos] <= 'z') || (p.input[p.pos] >= 'A' && p.input[p.pos] <= 'Z') || p.input[p.pos] == '-') {
			p.pos++
		}
		lang := p.input[langStart:p.pos]
		return NewLiteralWithLanguage(value.String(), lang), nil
	}

	if p.pos+1 < p.length && p.input[p.pos] == '^' && p.input[p.pos+1] == '^' {
		// Datatype - can be either an IRI or a prefixed name
		p.pos += 2 // skip '^^'
		datatypeTerm, err := p.parseTerm()
		if err != nil {
			return nil, fmt.Errorf("failed to parse datatype: %w", err)
		}
		// datatypeTerm should be a NamedNode
		if namedNode, ok := datatypeTerm.(*NamedNode); ok {
			return NewLiteralWithDatatype(value.String(), namedNode), nil
		}
		return nil, fmt.Errorf("datatype must be an IRI or prefixed name")
	}

	return NewLiteral(value.String()), nil
}

// parseNumber parses a numeric literal
func (p *TurtleParser) parseNumber() (Term, error) {
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

// parsePrefixedName parses a prefixed name (e.g., ex:foo or :foo)
func (p *TurtleParser) parsePrefixedName() (Term, error) {
	start := p.pos

	// Read prefix (until ':')
	for p.pos < p.length && p.input[p.pos] != ':' {
		ch := p.input[p.pos]
		if !isNameChar(ch) {
			break
		}
		p.pos++
	}

	if p.pos >= p.length || p.input[p.pos] != ':' {
		return nil, fmt.Errorf("expected ':' in prefixed name")
	}

	prefix := p.input[start:p.pos]
	p.pos++ // skip ':'

	// Read local part - can contain colons and many other characters per Turtle spec
	localStart := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		// Local names can contain alphanumeric, underscore, hyphen, colon, and many other chars
		// Break on whitespace, punctuation that ends a triple, or special Turtle syntax
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' ||
		   ch == '.' || ch == ';' || ch == ',' || ch == '>' || ch == '<' ||
		   ch == '"' || ch == '#' {
			break
		}
		p.pos++
	}

	localPart := p.input[localStart:p.pos]

	// Expand prefix
	baseIRI, ok := p.prefixes[prefix]
	if !ok {
		return nil, fmt.Errorf("undefined prefix: '%s'", prefix)
	}

	fullIRI := baseIRI + localPart
	return NewNamedNode(fullIRI), nil
}
