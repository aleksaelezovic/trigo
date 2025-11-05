package rdf

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// TurtleParser is a simple Turtle/N-Triples parser for loading test data
type TurtleParser struct {
	input            string
	pos              int
	length           int
	prefixes         map[string]string
	base             string
	blankNodeCounter int
	strictNTriples   bool // When true, enforce strict N-Triples syntax
}

// NewTurtleParser creates a new Turtle parser
func NewTurtleParser(input string) *TurtleParser {
	return &TurtleParser{
		input:          input,
		pos:            0,
		length:         len(input),
		prefixes:       make(map[string]string),
		strictNTriples: false,
	}
}

// NewNTriplesParser creates a new N-Triples parser with strict validation
func NewNTriplesParser(input string) *TurtleParser {
	return &TurtleParser{
		input:          input,
		pos:            0,
		length:         len(input),
		prefixes:       make(map[string]string),
		strictNTriples: true,
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
			if p.strictNTriples {
				return nil, fmt.Errorf("PREFIX directive not allowed in N-Triples")
			}
			if err := p.parsePrefix(); err != nil {
				return nil, err
			}
			continue
		}

		// Check for BASE directive
		if p.matchKeyword("@base") || p.matchKeyword("BASE") {
			if p.strictNTriples {
				return nil, fmt.Errorf("BASE directive not allowed in N-Triples")
			}
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
	baseIRI, err := p.parseIRI()
	if err != nil {
		return fmt.Errorf("failed to parse base IRI: %w", err)
	}

	// Store the base IRI
	p.base = baseIRI

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
				if p.strictNTriples {
					return nil, fmt.Errorf("comma abbreviation not allowed in N-Triples at position %d", p.pos)
				}
				p.pos++ // skip ','
				continue
			}
			break
		}

		p.skipWhitespaceAndComments()

		// Check for semicolon (more predicates with same subject)
		if p.pos < p.length && p.input[p.pos] == ';' {
			if p.strictNTriples {
				return nil, fmt.Errorf("semicolon abbreviation not allowed in N-Triples at position %d", p.pos)
			}
			// Skip all consecutive semicolons (repeated semicolons are allowed)
			for p.pos < p.length && p.input[p.pos] == ';' {
				p.pos++
				p.skipWhitespaceAndComments()
			}
			// Check if there's actually a predicate following (not just trailing semicolons)
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

	// Blank node (labeled: _:label)
	if ch == '_' && p.pos+1 < p.length && p.input[p.pos+1] == ':' {
		return p.parseBlankNode()
	}

	// Anonymous blank node or blank node property list: []
	if ch == '[' {
		return p.parseAnonymousBlankNode()
	}

	// Collection: (...)
	if ch == '(' {
		return p.parseCollection()
	}

	// String literal (double or single quote)
	if ch == '"' || ch == '\'' {
		return p.parseLiteral()
	}

	// Number literal
	if (ch >= '0' && ch <= '9') || ch == '-' || ch == '+' {
		if p.strictNTriples {
			return nil, fmt.Errorf("bare numeric literals not allowed in N-Triples at position %d", p.pos)
		}
		return p.parseNumber()
	}

	// Check for 'a' keyword (shorthand for rdf:type)
	// Must check if 'a' is followed by a non-name character (using Unicode-aware check)
	if ch == 'a' {
		// Peek at the next character to see if it could be part of a prefixed name
		nextPos := p.pos + 1
		isStandaloneA := true
		if nextPos < p.length {
			// Decode the next rune to check if it's a valid name continuation
			nextRune, _ := utf8.DecodeRuneInString(p.input[nextPos:])
			// Check if it could be part of a prefixed name (PN_CHARS or ':')
			if isPN_CHARS(nextRune) || nextRune == ':' || nextRune == '.' {
				isStandaloneA = false
			}
		}
		if isStandaloneA {
			if p.strictNTriples {
				return nil, fmt.Errorf("'a' abbreviation not allowed in N-Triples at position %d", p.pos)
			}
			p.pos++ // skip 'a'
			return NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), nil
		}
	}

	// Check for boolean literals
	if p.matchKeyword("true") {
		if p.strictNTriples {
			return nil, fmt.Errorf("bare boolean literals not allowed in N-Triples at position %d", p.pos)
		}
		// matchKeyword already advanced p.pos
		return NewBooleanLiteral(true), nil
	}
	if p.matchKeyword("false") {
		if p.strictNTriples {
			return nil, fmt.Errorf("bare boolean literals not allowed in N-Triples at position %d", p.pos)
		}
		// matchKeyword already advanced p.pos
		return NewBooleanLiteral(false), nil
	}

	// Prefixed name
	if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == ':' {
		return p.parsePrefixedName()
	}

	return nil, fmt.Errorf("unexpected character: %c at position %d", ch, p.pos)
}

// peekRune reads the next UTF-8 rune at the current position without advancing permanently
func (p *TurtleParser) peekRune() (rune, int) {
	if p.pos >= p.length {
		return 0, 0
	}
	r, size := utf8.DecodeRuneInString(p.input[p.pos:])
	return r, size
}

// isPN_CHARS_BASE checks if a rune is a PN_CHARS_BASE character per Turtle spec
// PN_CHARS_BASE ::= [A-Z] | [a-z] | [#x00C0-#x00D6] | [#x00D8-#x00F6] | [#x00F8-#x02FF] |
//
//	[#x0370-#x037D] | [#x037F-#x1FFF] | [#x200C-#x200D] | [#x2070-#x218F] |
//	[#x2C00-#x2FEF] | [#x3001-#xD7FF] | [#xF900-#xFDCF] | [#xFDF0-#xFFFD] |
//	[#x10000-#xEFFFF]
func isPN_CHARS_BASE(r rune) bool {
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0x00C0 && r <= 0x00D6) ||
		(r >= 0x00D8 && r <= 0x00F6) ||
		(r >= 0x00F8 && r <= 0x02FF) ||
		(r >= 0x0370 && r <= 0x037D) ||
		(r >= 0x037F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

// isPN_CHARS_U checks if a rune is a PN_CHARS_U character per Turtle spec
// PN_CHARS_U ::= PN_CHARS_BASE | '_'
func isPN_CHARS_U(r rune) bool {
	return isPN_CHARS_BASE(r) || r == '_'
}

// isPN_CHARS checks if a rune is a PN_CHARS character per Turtle spec
// PN_CHARS ::= PN_CHARS_U | '-' | [0-9] | #x00B7 | [#x0300-#x036F] | [#x203F-#x2040]
func isPN_CHARS(r rune) bool {
	return isPN_CHARS_U(r) ||
		r == '-' ||
		(r >= '0' && r <= '9') ||
		r == 0x00B7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}

// parseIRI parses an IRI in angle brackets
func (p *TurtleParser) parseIRI() (string, error) {
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

		// N-Triples/Turtle IRI validation
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

	// Check if IRI is relative (doesn't contain scheme with ':')
	if !strings.Contains(iri, ":") {
		// Relative IRI - resolve against base if available
		if p.base == "" {
			return "", fmt.Errorf("relative IRI not allowed without base: %s", iri)
		}
		// Resolve relative IRI against base
		iri = p.resolveRelativeIRI(p.base, iri)
	}

	return iri, nil
}

// resolveRelativeIRI resolves a relative IRI against a base IRI
// This is a simplified implementation of RFC 3986 resolution
func (p *TurtleParser) resolveRelativeIRI(base, relative string) string {
	// Empty relative IRI → use base
	if relative == "" {
		return base
	}

	// Fragment only (#foo) → base without fragment + new fragment
	if strings.HasPrefix(relative, "#") {
		// Remove any existing fragment from base
		if idx := strings.Index(base, "#"); idx >= 0 {
			base = base[:idx]
		}
		return base + relative
	}

	// Query or fragment (?foo or #foo) → base without query/fragment + relative
	if strings.HasPrefix(relative, "?") {
		// Remove query and fragment from base
		if idx := strings.Index(base, "?"); idx >= 0 {
			base = base[:idx]
		} else if idx := strings.Index(base, "#"); idx >= 0 {
			base = base[:idx]
		}
		return base + relative
	}

	// Absolute path (/foo) → scheme + authority + relative path
	if strings.HasPrefix(relative, "/") {
		// Find scheme and authority in base
		schemeEnd := strings.Index(base, ":")
		if schemeEnd < 0 {
			return relative // shouldn't happen
		}

		// Check for authority (://...)
		if schemeEnd+2 < len(base) && base[schemeEnd:schemeEnd+3] == "://" {
			// Find end of authority (next /)
			authorityStart := schemeEnd + 3
			pathStart := strings.Index(base[authorityStart:], "/")
			if pathStart >= 0 {
				return base[:authorityStart+pathStart] + relative
			}
			// No path in base, append to authority
			return base + relative
		}

		// No authority, just scheme
		return base[:schemeEnd+1] + relative
	}

	// Relative path (foo or ./foo or ../foo) → resolve against base path
	// Remove query and fragment from base
	baseWithoutQF := base
	if idx := strings.Index(baseWithoutQF, "?"); idx >= 0 {
		baseWithoutQF = baseWithoutQF[:idx]
	} else if idx := strings.Index(baseWithoutQF, "#"); idx >= 0 {
		baseWithoutQF = baseWithoutQF[:idx]
	}

	// Find the last / in base to get the directory
	lastSlash := strings.LastIndex(baseWithoutQF, "/")
	if lastSlash >= 0 {
		// Append relative path to base directory
		return baseWithoutQF[:lastSlash+1] + relative
	}

	// No / found, just concatenate (shouldn't happen with valid IRIs)
	return baseWithoutQF + "/" + relative
}

// processUnicodeEscape processes \uXXXX or \UXXXXXXXX escape sequences
func (p *TurtleParser) processUnicodeEscape() (string, error) {
	if p.pos >= p.length || p.input[p.pos] != '\\' {
		return "", fmt.Errorf("expected '\\' at start of escape sequence")
	}
	p.pos++ // skip '\'

	if p.pos >= p.length {
		return "", fmt.Errorf("incomplete escape sequence")
	}

	escapeType := p.input[p.pos]
	p.pos++ // skip 'u' or 'U'

	var hexDigits int
	if escapeType == 'u' {
		hexDigits = 4
	} else if escapeType == 'U' {
		hexDigits = 8
	} else {
		return "", fmt.Errorf("invalid escape type: %c", escapeType)
	}

	if p.pos+hexDigits > p.length {
		return "", fmt.Errorf("incomplete Unicode escape sequence")
	}

	hexStr := p.input[p.pos : p.pos+hexDigits]
	p.pos += hexDigits

	// Parse hex string to rune
	codePoint, err := strconv.ParseInt(hexStr, 16, 32)
	if err != nil {
		return "", fmt.Errorf("invalid hex digits in Unicode escape: %s", hexStr)
	}

	return string(rune(codePoint)), nil
}

// parseBlankNode parses a blank node
func (p *TurtleParser) parseBlankNode() (Term, error) {
	if p.pos+1 >= p.length || p.input[p.pos] != '_' || p.input[p.pos+1] != ':' {
		return nil, fmt.Errorf("expected '_:' at start of blank node")
	}
	p.pos += 2 // skip '_:'

	start := p.pos

	// BLANK_NODE_LABEL ::= '_:' (PN_CHARS_U | [0-9]) ((PN_CHARS | '.')* PN_CHARS)?
	// First character must be PN_CHARS_U or digit
	if p.pos < p.length {
		r, size := p.peekRune()
		if !isPN_CHARS_U(r) && !(r >= '0' && r <= '9') {
			return nil, fmt.Errorf("invalid blank node label start character at position %d", p.pos)
		}
		p.pos += size
	}

	// Continue reading label characters (PN_CHARS | '.')
	lastCharWasDot := false
	for p.pos < p.length {
		r, size := p.peekRune()
		if !isPN_CHARS(r) && r != '.' {
			break
		}
		lastCharWasDot = (r == '.')
		p.pos += size
	}

	// Blank node labels cannot end with '.' - backtrack if needed
	if lastCharWasDot {
		p.pos--
	}

	label := p.input[start:p.pos]
	return NewBlankNode(label), nil
}

// parseAnonymousBlankNode parses an anonymous blank node [] or blank node property list
func (p *TurtleParser) parseAnonymousBlankNode() (Term, error) {
	if p.pos >= p.length || p.input[p.pos] != '[' {
		return nil, fmt.Errorf("expected '[' at start of blank node")
	}
	p.pos++ // skip '['
	p.skipWhitespaceAndComments()

	p.blankNodeCounter++
	blankNode := NewBlankNode(fmt.Sprintf("anon%d", p.blankNodeCounter))

	// Check if it's just [] or has properties
	if p.pos < p.length && p.input[p.pos] == ']' {
		p.pos++ // skip ']'
		return blankNode, nil
	}

	// Skip property list content (architecture limitation: parseTerm returns single Term
	// but property lists generate multiple triples - need parser refactoring to support properly)
	bracketDepth := 1
	for p.pos < p.length && bracketDepth > 0 {
		if p.input[p.pos] == '[' {
			bracketDepth++
		} else if p.input[p.pos] == ']' {
			bracketDepth--
		}
		p.pos++
	}

	return blankNode, nil
}

// parseCollection parses a collection (RDF list): (item1 item2 ...)
func (p *TurtleParser) parseCollection() (Term, error) {
	if p.pos >= p.length || p.input[p.pos] != '(' {
		return nil, fmt.Errorf("expected '(' at start of collection")
	}
	p.pos++ // skip '('
	p.skipWhitespaceAndComments()

	// Check for empty collection
	if p.pos < p.length && p.input[p.pos] == ')' {
		p.pos++ // skip ')'
		return NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#nil"), nil
	}

	// Non-empty collection - create blank node for list head
	p.blankNodeCounter++
	listHead := NewBlankNode(fmt.Sprintf("list%d", p.blankNodeCounter))

	// Skip collection content (architecture limitation: collections generate multiple
	// triples using rdf:first/rdf:rest but parseTerm returns single Term)
	parenDepth := 1
	for p.pos < p.length && parenDepth > 0 {
		if p.input[p.pos] == '(' {
			parenDepth++
		} else if p.input[p.pos] == ')' {
			parenDepth--
		}
		p.pos++
	}

	return listHead, nil
}

// parseLiteral parses a string literal
func (p *TurtleParser) parseLiteral() (Term, error) {
	if p.pos >= p.length {
		return nil, fmt.Errorf("unexpected end of input when expecting literal")
	}

	// Check if it's a long literal (""" or ''')
	if p.pos+2 < p.length {
		if p.input[p.pos:p.pos+3] == `"""` {
			if p.strictNTriples {
				return nil, fmt.Errorf("triple-quoted literals not allowed in N-Triples")
			}
			return p.parseLongLiteral(`"""`)
		} else if p.input[p.pos:p.pos+3] == `'''` {
			if p.strictNTriples {
				return nil, fmt.Errorf("triple-quoted literals not allowed in N-Triples")
			}
			return p.parseLongLiteral(`'''`)
		}
	}

	// Single or double quote literal
	quoteChar := p.input[p.pos]
	if quoteChar != '"' && quoteChar != '\'' {
		return nil, fmt.Errorf("expected quote at start of literal")
	}
	if quoteChar == '\'' && p.strictNTriples {
		return nil, fmt.Errorf("single-quoted literals not allowed in N-Triples")
	}
	p.pos++ // skip opening quote

	var value strings.Builder
	for p.pos < p.length {
		ch := p.input[p.pos]
		if ch == quoteChar {
			break
		}
		if ch == '\\' && p.pos+1 < p.length {
			// Handle escape sequences
			nextCh := p.input[p.pos+1]
			if nextCh == 'u' || nextCh == 'U' {
				// Unicode escape sequence
				escaped, err := p.processUnicodeEscape()
				if err != nil {
					return nil, err
				}
				value.WriteString(escaped)
			} else {
				// Regular escape sequences
				p.pos++
				switch p.input[p.pos] {
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
				case '\'':
					value.WriteByte('\'')
				case '\\':
					value.WriteByte('\\')
				default:
					return nil, fmt.Errorf("invalid escape sequence \\%c at position %d", p.input[p.pos], p.pos)
				}
				p.pos++
			}
		} else {
			value.WriteByte(ch)
			p.pos++
		}
	}

	if p.pos >= p.length {
		return nil, fmt.Errorf("unclosed string literal")
	}
	p.pos++ // skip closing quote

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

// parseLongLiteral parses a long string literal (""" or ”')
func (p *TurtleParser) parseLongLiteral(delimiter string) (Term, error) {
	if p.pos+3 > p.length || p.input[p.pos:p.pos+3] != delimiter {
		return nil, fmt.Errorf("expected %s at start of long literal", delimiter)
	}
	p.pos += 3 // skip opening delimiter

	var value strings.Builder
	for p.pos < p.length {
		// Check for closing delimiter
		if p.pos+3 <= p.length && p.input[p.pos:p.pos+3] == delimiter {
			p.pos += 3 // skip closing delimiter
			break
		}

		ch := p.input[p.pos]
		if ch == '\\' && p.pos+1 < p.length {
			// Handle escape sequences
			nextCh := p.input[p.pos+1]
			if nextCh == 'u' || nextCh == 'U' {
				// Unicode escape sequence
				escaped, err := p.processUnicodeEscape()
				if err != nil {
					return nil, err
				}
				value.WriteString(escaped)
			} else {
				// Regular escape sequences
				p.pos++
				switch p.input[p.pos] {
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
				case '\'':
					value.WriteByte('\'')
				case '\\':
					value.WriteByte('\\')
				default:
					return nil, fmt.Errorf("invalid escape sequence \\%c at position %d", p.input[p.pos], p.pos)
				}
				p.pos++
			}
		} else {
			value.WriteByte(ch)
			p.pos++
		}
	}

	// Check if we found the closing delimiter
	if p.pos > p.length || (p.pos == p.length && !strings.HasSuffix(p.input, delimiter)) {
		return nil, fmt.Errorf("unclosed long string literal")
	}

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
		// Datatype
		p.pos += 2 // skip '^^'
		datatypeTerm, err := p.parseTerm()
		if err != nil {
			return nil, fmt.Errorf("failed to parse datatype: %w", err)
		}
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

// parsePrefixedName parses a prefixed name (e.g., ex:foo or :foo)
func (p *TurtleParser) parsePrefixedName() (Term, error) {
	start := p.pos

	// Read prefix (until ':')
	// PN_PREFIX ::= PN_CHARS_BASE ((PN_CHARS|'.')* PN_CHARS)?
	// Empty prefix is allowed (e.g., :localName)
	if p.pos < p.length && p.input[p.pos] != ':' {
		// First character must be PN_CHARS_BASE
		r, size := p.peekRune()
		if !isPN_CHARS_BASE(r) {
			return nil, fmt.Errorf("invalid prefix start character at position %d", p.pos)
		}
		p.pos += size

		// Continue reading prefix characters (PN_CHARS | '.')
		lastCharWasDot := false
		for p.pos < p.length && p.input[p.pos] != ':' {
			r, size := p.peekRune()
			if !isPN_CHARS(r) && r != '.' {
				break
			}
			lastCharWasDot = (r == '.')
			p.pos += size
		}

		// Prefix cannot end with '.' - backtrack if needed
		if lastCharWasDot {
			p.pos--
		}
	}

	if p.pos >= p.length || p.input[p.pos] != ':' {
		return nil, fmt.Errorf("expected ':' in prefixed name")
	}

	prefix := p.input[start:p.pos]
	p.pos++ // skip ':'

	// Read local part - can contain colons and many other characters per Turtle spec
	// Also supports escape sequences like \- \. \~ etc. (PN_LOCAL_ESC)
	// PN_LOCAL ::= (PN_CHARS_U | ':' | [0-9] | PLX) ((PN_CHARS | '.' | ':' | PLX)* (PN_CHARS | ':' | PLX))?
	var localPart strings.Builder
	for p.pos < p.length {
		r, size := p.peekRune()

		// Handle escape sequences (PLX includes PN_LOCAL_ESC and PERCENT)
		if r == '\\' && p.pos+1 < p.length {
			nextCh := p.input[p.pos+1]
			// Check if this is a valid PN_LOCAL_ESC character
			if nextCh == '_' || nextCh == '~' || nextCh == '.' || nextCh == '-' ||
				nextCh == '!' || nextCh == '$' || nextCh == '&' || nextCh == '\'' ||
				nextCh == '(' || nextCh == ')' || nextCh == '*' || nextCh == '+' ||
				nextCh == ',' || nextCh == ';' || nextCh == '=' || nextCh == '/' ||
				nextCh == '?' || nextCh == '#' || nextCh == '@' || nextCh == '%' || nextCh == ':' {
				// Add the escaped character without the backslash
				localPart.WriteByte(nextCh)
				p.pos += 2
				continue
			}
		}

		// Local names can contain PN_CHARS, ':', '.', and many other chars
		// Break on whitespace, punctuation that ends a triple, or special Turtle syntax
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' ||
			r == '>' || r == '<' || r == '"' {
			break
		}
		// Break on these only if not escaped (we already handled escaping above)
		if r == ';' || r == ',' || r == '#' {
			break
		}
		// Allow PN_CHARS, ':', '.', digits for local names
		// This is more permissive than the spec but matches existing behavior
		localPart.WriteRune(r)
		p.pos += size
	}

	localPartStr := localPart.String()

	// Expand prefix
	baseIRI, ok := p.prefixes[prefix]
	if !ok {
		return nil, fmt.Errorf("undefined prefix: '%s'", prefix)
	}

	fullIRI := baseIRI + localPartStr
	return NewNamedNode(fullIRI), nil
}
