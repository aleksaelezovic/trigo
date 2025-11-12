package rdf

import (
	"fmt"
	"strings"
)

// NQuadsParser is an N-Quads parser that extends N-Triples with an optional 4th position for graphs
// N-Quads format: <subject> <predicate> <object> [<graph>] .
// Compatible with N-Triples (3 positions) - defaults to default graph
type NQuadsParser struct {
	input      string
	pos        int
	length     int
	prefixes   map[string]string
	baseIRI    string
	strictMode bool // When true, enforce strict N-Quads/N-Triples syntax
}

// NewNQuadsParser creates a new N-Quads parser with strict validation
func NewNQuadsParser(input string) *NQuadsParser {
	return &NQuadsParser{
		input:      input,
		pos:        0,
		length:     len(input),
		prefixes:   make(map[string]string),
		strictMode: true, // N-Quads uses strict N-Triples syntax
	}
}

// Parse parses the N-Quads document and returns quads
func (p *NQuadsParser) Parse() ([]*Quad, error) {
	var quads []*Quad

	for p.pos < p.length {
		p.skipWhitespaceAndComments()
		if p.pos >= p.length {
			break
		}

		// Check for PREFIX directive (optional Turtle extension)
		if p.matchKeyword("@prefix") || p.matchKeyword("PREFIX") {
			if p.strictMode {
				return nil, fmt.Errorf("PREFIX directive not allowed in N-Quads")
			}
			if err := p.parsePrefix(); err != nil {
				return nil, err
			}
			continue
		}

		// Check for BASE directive (optional Turtle extension)
		if p.matchKeyword("@base") || p.matchKeyword("BASE") {
			if p.strictMode {
				return nil, fmt.Errorf("BASE directive not allowed in N-Quads")
			}
			if err := p.parseBase(); err != nil {
				return nil, err
			}
			continue
		}

		// Parse quad (or triple as quad in default graph)
		quad, err := p.parseQuad()
		if err != nil {
			return nil, err
		}
		if quad != nil {
			quads = append(quads, quad)
		}
	}

	return quads, nil
}

// skipWhitespaceAndComments skips whitespace and comments
func (p *NQuadsParser) skipWhitespaceAndComments() {
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
func (p *NQuadsParser) matchKeyword(keyword string) bool {
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
		if nextCh != ' ' && nextCh != '\t' && nextCh != '\n' && nextCh != '\r' {
			return false
		}
	}

	return true
}

// parsePrefix parses a PREFIX directive
func (p *NQuadsParser) parsePrefix() error {
	// Skip "PREFIX" or "@prefix"
	for p.pos < p.length && p.input[p.pos] != ' ' && p.input[p.pos] != '\t' {
		p.pos++
	}
	p.skipWhitespaceAndComments()

	// Parse prefix name
	start := p.pos
	for p.pos < p.length && p.input[p.pos] != ':' {
		p.pos++
	}
	if p.pos >= p.length {
		return fmt.Errorf("expected ':' after prefix name")
	}
	prefixName := strings.TrimSpace(p.input[start:p.pos])
	p.pos++ // skip ':'

	p.skipWhitespaceAndComments()

	// Parse IRI
	iri, err := p.parseIRI()
	if err != nil {
		return fmt.Errorf("error parsing prefix IRI: %w", err)
	}

	p.prefixes[prefixName] = iri

	// Skip optional '.' at end
	p.skipWhitespaceAndComments()
	if p.pos < p.length && p.input[p.pos] == '.' {
		p.pos++
	}

	return nil
}

// parseBase parses a BASE directive
func (p *NQuadsParser) parseBase() error {
	// Skip "BASE" or "@base"
	for p.pos < p.length && p.input[p.pos] != ' ' && p.input[p.pos] != '\t' {
		p.pos++
	}
	p.skipWhitespaceAndComments()

	// Parse base IRI
	iri, err := p.parseIRI()
	if err != nil {
		return fmt.Errorf("error parsing base IRI: %w", err)
	}

	p.baseIRI = iri

	// Skip optional '.' at end
	p.skipWhitespaceAndComments()
	if p.pos < p.length && p.input[p.pos] == '.' {
		p.pos++
	}

	return nil
}

// parseQuad parses a quad: subject predicate object [graph] .
func (p *NQuadsParser) parseQuad() (*Quad, error) {
	// Parse subject
	subject, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("error parsing subject: %w", err)
	}

	// RDF 1.2 N-Quads: Triple terms (quoted triples) cannot be used as subjects
	if _, ok := subject.(*QuotedTriple); ok {
		return nil, fmt.Errorf("triple terms cannot be used as subjects in N-Quads")
	}

	p.skipWhitespaceAndComments()

	// Parse predicate
	predicate, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("error parsing predicate: %w", err)
	}

	// RDF 1.2 N-Quads: Triple terms (quoted triples) cannot be used as predicates
	if _, ok := predicate.(*QuotedTriple); ok {
		return nil, fmt.Errorf("triple terms cannot be used as predicates in N-Quads")
	}

	p.skipWhitespaceAndComments()

	// Parse object
	object, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("error parsing object: %w", err)
	}
	// Note: Triple terms ARE allowed as objects

	p.skipWhitespaceAndComments()

	// Parse optional graph (4th position)
	var graph Term
	if p.pos < p.length && p.input[p.pos] == '<' {
		// Graph IRI
		graph, err = p.parseTerm()
		if err != nil {
			return nil, fmt.Errorf("error parsing graph: %w", err)
		}
		p.skipWhitespaceAndComments()
	} else if p.pos < p.length && p.input[p.pos] == '_' {
		// Graph blank node
		graph, err = p.parseBlankNode()
		if err != nil {
			return nil, fmt.Errorf("error parsing graph: %w", err)
		}
		p.skipWhitespaceAndComments()
	}

	// Expect '.' at end
	if p.pos >= p.length || p.input[p.pos] != '.' {
		return nil, fmt.Errorf("expected '.' at end of quad")
	}
	p.pos++ // skip '.'

	// Create quad
	var quad *Quad
	if graph == nil {
		// No graph specified - use default graph
		quad = NewQuad(subject, predicate, object, NewDefaultGraph())
	} else {
		quad = NewQuad(subject, predicate, object, graph)
	}

	return quad, nil
}

// parseTerm parses an RDF term (IRI, blank node, literal, or quoted triple)
func (p *NQuadsParser) parseTerm() (Term, error) {
	ch := p.input[p.pos]

	switch ch {
	case '<':
		// Could be IRI or quoted triple
		// Check for <<( which indicates quoted triple (RDF 1.2 N-Triples)
		if strings.HasPrefix(p.input[p.pos:], "<<(") {
			return p.parseQuotedTriple()
		}
		// IRI
		iri, err := p.parseIRI()
		if err != nil {
			return nil, err
		}
		return NewNamedNode(iri), nil

	case '_':
		// Blank node
		return p.parseBlankNode()

	case '"':
		// Literal
		return p.parseLiteral()

	case '-', '+', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// Numeric literal
		if p.strictMode {
			return nil, fmt.Errorf("bare numeric literals not allowed in N-Quads at position %d", p.pos)
		}
		return p.parseNumber()

	default:
		// Check for prefixed name
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			return p.parsePrefixedName()
		}
		return nil, fmt.Errorf("unexpected character at position %d: %c", p.pos, ch)
	}
}

// parseIRI parses an IRI enclosed in < >
func (p *NQuadsParser) parseIRI() (string, error) {
	if p.pos >= p.length || p.input[p.pos] != '<' {
		return "", fmt.Errorf("expected '<' at start of IRI")
	}
	p.pos++ // skip '<'

	var result strings.Builder
	for p.pos < p.length && p.input[p.pos] != '>' {
		ch := p.input[p.pos]

		// Handle Unicode escape sequences
		if ch == '\\' {
			if p.pos+1 < p.length {
				nextCh := p.input[p.pos+1]
				if nextCh == 'u' || nextCh == 'U' {
					// Process Unicode escape
					escaped, err := p.processUnicodeEscape()
					if err != nil {
						return "", err
					}
					result.WriteString(escaped)
					continue
				}
			}
			// Backslash not followed by u/U is invalid in IRIs
			return "", fmt.Errorf("invalid escape sequence in IRI at position %d", p.pos)
		}

		// N-Quads/N-Triples IRI validation
		// IRIs cannot contain: space, <, >, ", {, }, |, ^, `
		// and must not contain control characters (0x00-0x1F)
		if ch == ' ' || ch == '<' || ch == '>' || ch == '"' || ch == '{' || ch == '}' ||
			ch == '|' || ch == '^' || ch == '`' || ch <= 0x1F {
			return "", fmt.Errorf("invalid character in IRI: %q at position %d", ch, p.pos)
		}

		result.WriteByte(ch)
		p.pos++
	}

	if p.pos >= p.length {
		return "", fmt.Errorf("unclosed IRI")
	}

	iri := result.String()
	p.pos++ // skip '>'

	// Validate that IRI has a scheme (contains ':')
	if !strings.Contains(iri, ":") {
		return "", fmt.Errorf("relative IRI not allowed in N-Quads: %s", iri)
	}

	return iri, nil
}

// parseBlankNode parses a blank node
func (p *NQuadsParser) parseBlankNode() (Term, error) {
	if p.pos >= p.length || p.input[p.pos] != '_' {
		return nil, fmt.Errorf("expected '_' at start of blank node")
	}
	p.pos++ // skip '_'

	if p.pos >= p.length || p.input[p.pos] != ':' {
		return nil, fmt.Errorf("expected ':' after '_' in blank node")
	}
	p.pos++ // skip ':'

	start := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '.' || ch == '<' {
			break
		}
		p.pos++
	}

	label := p.input[start:p.pos]
	return NewBlankNode(label), nil
}

// parseLiteral parses a literal value
func (p *NQuadsParser) parseLiteral() (Term, error) {
	if p.pos >= p.length || p.input[p.pos] != '"' {
		return nil, fmt.Errorf("expected '\"' at start of literal")
	}
	p.pos++ // skip opening '"'

	var value strings.Builder
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == '"' {
			break
		}
		if ch == '\\' {
			// Handle escape sequences
			p.pos++
			if p.pos >= p.length {
				return nil, fmt.Errorf("unexpected end of input in escape sequence")
			}
			escCh := p.input[p.pos]
			switch escCh {
			case 'n':
				value.WriteByte('\n')
			case 't':
				value.WriteByte('\t')
			case 'r':
				value.WriteByte('\r')
			case 'b':
				value.WriteByte('\b')
			case 'f':
				value.WriteByte('\f')
			case '"':
				value.WriteByte('"')
			case '\\':
				value.WriteByte('\\')
			case 'u', 'U':
				// Unicode escape - go back and let processUnicodeEscape handle it
				p.pos--
				escaped, err := p.processUnicodeEscape()
				if err != nil {
					return nil, err
				}
				value.WriteString(escaped)
				continue
			default:
				return nil, fmt.Errorf("invalid escape sequence \\%c at position %d", escCh, p.pos)
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
	if p.pos < p.length {
		if p.input[p.pos] == '@' {
			// Language tag (with optional direction suffix for RDF 1.2)
			p.pos++ // skip '@'
			start := p.pos
			if p.pos >= p.length {
				return nil, fmt.Errorf("empty language tag")
			}
			// Language tags must start with a letter (BCP 47)
			firstChar := p.input[p.pos]
			if !((firstChar >= 'a' && firstChar <= 'z') || (firstChar >= 'A' && firstChar <= 'Z')) {
				return nil, fmt.Errorf("invalid language tag: must start with a letter, got %q", firstChar)
			}
			for p.pos < p.length {
				ch := p.input[p.pos]
				if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '.' || ch == '<' {
					break
				}
				p.pos++
			}
			langTag := p.input[start:p.pos]

			// Check for direction suffix: --ltr or --rtl (RDF 1.2)
			if strings.Contains(langTag, "--") {
				idx := strings.Index(langTag, "--")
				lang := langTag[:idx]
				dir := langTag[idx+2:]

				// Direction must be present after --
				if dir == "" {
					return nil, fmt.Errorf("missing direction after '--' in language tag")
				}

				// Language tag must be present before --
				if lang == "" {
					return nil, fmt.Errorf("missing language tag before '--' in language tag")
				}

				// Direction must be exactly "ltr" or "rtl" (lowercase only)
				if dir != "ltr" && dir != "rtl" {
					return nil, fmt.Errorf("invalid direction in language tag: %q (must be 'ltr' or 'rtl', lowercase)", dir)
				}

				return NewLiteralWithLanguageAndDirection(value.String(), lang, dir), nil
			}

			return NewLiteralWithLanguage(value.String(), langTag), nil
		} else if p.input[p.pos] == '^' && p.pos+1 < p.length && p.input[p.pos+1] == '^' {
			// Datatype
			p.pos += 2 // skip '^^'
			p.skipWhitespaceAndComments()
			datatypeIRI, err := p.parseIRI()
			if err != nil {
				return nil, fmt.Errorf("error parsing datatype: %w", err)
			}
			return NewLiteralWithDatatype(value.String(), NewNamedNode(datatypeIRI)), nil
		}
	}

	// Plain literal
	return NewLiteral(value.String()), nil
}

// parseQuotedTriple parses an RDF 1.2 N-Triples quoted triple: <<( subject predicate object )>>
func (p *NQuadsParser) parseQuotedTriple() (*QuotedTriple, error) {
	// Expect '<<('
	if p.pos+2 >= p.length || p.input[p.pos:p.pos+3] != "<<(" {
		return nil, fmt.Errorf("expected '<<(' at start of quoted triple")
	}
	p.pos += 3 // skip '<<('

	p.skipWhitespaceAndComments()

	// Parse subject (can be IRI, blank node, or nested quoted triple)
	subject, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("error parsing quoted triple subject: %w", err)
	}

	// Validate: subject cannot be literal
	if _, ok := subject.(*Literal); ok {
		return nil, fmt.Errorf("quoted triple subject cannot be a literal")
	}

	p.skipWhitespaceAndComments()

	// Parse predicate (must be IRI)
	predicate, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("error parsing quoted triple predicate: %w", err)
	}

	// Validate: predicate must be IRI (not blank node, literal, or quoted triple)
	if _, ok := predicate.(*NamedNode); !ok {
		return nil, fmt.Errorf("quoted triple predicate must be an IRI, got %T", predicate)
	}

	p.skipWhitespaceAndComments()

	// Parse object (can be any term including quoted triple)
	object, err := p.parseTerm()
	if err != nil {
		return nil, fmt.Errorf("error parsing quoted triple object: %w", err)
	}

	p.skipWhitespaceAndComments()

	// Expect ')>>'
	if p.pos+2 >= p.length || p.input[p.pos:p.pos+3] != ")>>" {
		return nil, fmt.Errorf("expected ')>>' at end of quoted triple, got: %q", p.input[p.pos:min(p.pos+3, p.length)])
	}
	p.pos += 3 // skip ')>>'

	// Create quoted triple
	qt, err := NewQuotedTriple(subject, predicate, object)
	if err != nil {
		return nil, fmt.Errorf("error creating quoted triple: %w", err)
	}

	return qt, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// processUnicodeEscape processes \uXXXX or \UXXXXXXXX escape sequences
func (p *NQuadsParser) processUnicodeEscape() (string, error) {
	if p.pos >= p.length || p.input[p.pos] != '\\' {
		return "", fmt.Errorf("expected '\\' at start of escape sequence")
	}
	p.pos++ // skip '\'

	if p.pos >= p.length {
		return "", fmt.Errorf("unexpected end of input in Unicode escape")
	}

	escapeType := p.input[p.pos]
	p.pos++ // skip 'u' or 'U'

	var hexDigits int
	if escapeType == 'u' {
		hexDigits = 4
	} else if escapeType == 'U' {
		hexDigits = 8
	} else {
		return "", fmt.Errorf("invalid Unicode escape type: %c", escapeType)
	}

	if p.pos+hexDigits > p.length {
		return "", fmt.Errorf("incomplete Unicode escape sequence")
	}

	hexStr := p.input[p.pos : p.pos+hexDigits]
	p.pos += hexDigits

	var codePoint int64
	var err error
	codePoint, err = func(s string, base, bitSize int) (int64, error) {
		var result int64
		for i := 0; i < len(s); i++ {
			var digit int64
			ch := s[i]
			switch {
			case ch >= '0' && ch <= '9':
				digit = int64(ch - '0')
			case ch >= 'a' && ch <= 'f':
				digit = int64(ch-'a') + 10
			case ch >= 'A' && ch <= 'F':
				digit = int64(ch-'A') + 10
			default:
				return 0, fmt.Errorf("invalid hex character: %c", ch)
			}
			result = result*16 + digit
		}
		return result, nil
	}(hexStr, 16, 32)

	if err != nil {
		return "", fmt.Errorf("invalid hex digits in Unicode escape: %s", hexStr)
	}

	return string(rune(codePoint)), nil
}

// parseNumber parses a numeric literal
func (p *NQuadsParser) parseNumber() (Term, error) {
	start := p.pos

	// Optional sign
	if p.pos < p.length && (p.input[p.pos] == '-' || p.input[p.pos] == '+') {
		p.pos++
	}

	// Digits
	hasDigits := false
	for p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
		hasDigits = true
	}

	// Check for decimal point
	isDecimal := false
	if p.pos < p.length && p.input[p.pos] == '.' {
		isDecimal = true
		p.pos++
		for p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
			hasDigits = true
		}
	}

	// Check for exponent
	if p.pos < p.length && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		isDecimal = true
		p.pos++
		if p.pos < p.length && (p.input[p.pos] == '-' || p.input[p.pos] == '+') {
			p.pos++
		}
		for p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}

	if !hasDigits {
		return nil, fmt.Errorf("invalid number at position %d", start)
	}

	numStr := p.input[start:p.pos]

	if isDecimal {
		// Parse as double
		return NewLiteralWithDatatype(numStr, XSDDouble), nil
	} else {
		// Parse as integer
		return NewLiteralWithDatatype(numStr, XSDInteger), nil
	}
}

// parsePrefixedName parses a prefixed name (e.g., ex:foo)
func (p *NQuadsParser) parsePrefixedName() (Term, error) {
	start := p.pos

	// Parse prefix
	for p.pos < p.length && p.input[p.pos] != ':' {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '.' {
			return nil, fmt.Errorf("invalid character in prefixed name")
		}
		p.pos++
	}

	if p.pos >= p.length {
		return nil, fmt.Errorf("expected ':' in prefixed name")
	}

	prefix := p.input[start:p.pos]
	p.pos++ // skip ':'

	// Parse local name
	localStart := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '.' || ch == '<' || ch == '>' {
			break
		}
		p.pos++
	}

	localName := p.input[localStart:p.pos]

	// Expand prefix
	baseIRI, ok := p.prefixes[prefix]
	if !ok {
		return nil, fmt.Errorf("undefined prefix: %s", prefix)
	}

	fullIRI := baseIRI + localName
	return NewNamedNode(fullIRI), nil
}
