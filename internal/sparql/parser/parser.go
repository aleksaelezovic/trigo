package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// Parser parses SPARQL queries
type Parser struct {
	input    string
	pos      int
	length   int
	prefixes map[string]string // Maps prefix to IRI
}

// NewParser creates a new SPARQL parser
func NewParser(input string) *Parser {
	return &Parser{
		input:    input,
		pos:      0,
		length:   len(input),
		prefixes: make(map[string]string),
	}
}

// Parse parses a SPARQL query
func (p *Parser) Parse() (*Query, error) {
	p.skipWhitespace()

	// Skip PREFIX and BASE declarations
	for {
		p.skipWhitespace()
		if p.matchKeyword("PREFIX") {
			// Skip PREFIX prefix: <iri>
			if err := p.skipPrefix(); err != nil {
				return nil, err
			}
		} else if p.matchKeyword("BASE") {
			// Skip BASE <iri>
			if err := p.skipBase(); err != nil {
				return nil, err
			}
		} else {
			break
		}
	}

	// Determine query type
	queryType, err := p.parseQueryType()
	if err != nil {
		return nil, err
	}

	query := &Query{QueryType: queryType}

	switch queryType {
	case QueryTypeSelect:
		selectQuery, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		query.Select = selectQuery
	case QueryTypeAsk:
		askQuery, err := p.parseAsk()
		if err != nil {
			return nil, err
		}
		query.Ask = askQuery
	case QueryTypeConstruct:
		constructQuery, err := p.parseConstruct()
		if err != nil {
			return nil, err
		}
		query.Construct = constructQuery
	default:
		return nil, fmt.Errorf("query type not yet implemented: %v", queryType)
	}

	return query, nil
}

// parseQueryType determines the query type
func (p *Parser) parseQueryType() (QueryType, error) {
	p.skipWhitespace()

	if p.matchKeyword("SELECT") {
		return QueryTypeSelect, nil
	}
	if p.matchKeyword("CONSTRUCT") {
		return QueryTypeConstruct, nil
	}
	if p.matchKeyword("ASK") {
		return QueryTypeAsk, nil
	}
	if p.matchKeyword("DESCRIBE") {
		return QueryTypeDescribe, nil
	}

	return 0, fmt.Errorf("expected query type (SELECT, CONSTRUCT, ASK, DESCRIBE)")
}

// parseSelect parses a SELECT query
func (p *Parser) parseSelect() (*SelectQuery, error) {
	query := &SelectQuery{}

	// Parse DISTINCT (optional)
	if p.matchKeyword("DISTINCT") {
		query.Distinct = true
	}

	// Parse variables or *
	variables, err := p.parseProjection()
	if err != nil {
		return nil, err
	}
	query.Variables = variables

	// Parse WHERE clause (WHERE keyword is optional)
	p.matchKeyword("WHERE") // consume WHERE if present, but don't require it

	where, err := p.parseGraphPattern()
	if err != nil {
		return nil, err
	}
	query.Where = where

	// Parse optional ORDER BY
	if p.matchKeyword("ORDER") {
		if !p.matchKeyword("BY") {
			return nil, fmt.Errorf("expected BY after ORDER")
		}
		orderBy, err := p.parseOrderBy()
		if err != nil {
			return nil, err
		}
		query.OrderBy = orderBy
	}

	// Parse optional LIMIT
	if p.matchKeyword("LIMIT") {
		limit, err := p.parseInteger()
		if err != nil {
			return nil, err
		}
		query.Limit = &limit
	}

	// Parse optional OFFSET
	if p.matchKeyword("OFFSET") {
		offset, err := p.parseInteger()
		if err != nil {
			return nil, err
		}
		query.Offset = &offset
	}

	return query, nil
}

// parseAsk parses an ASK query
func (p *Parser) parseAsk() (*AskQuery, error) {
	query := &AskQuery{}

	// Parse WHERE clause
	if !p.matchKeyword("WHERE") {
		return nil, fmt.Errorf("expected WHERE clause")
	}

	where, err := p.parseGraphPattern()
	if err != nil {
		return nil, err
	}
	query.Where = where

	return query, nil
}

// parseConstruct parses a CONSTRUCT query
func (p *Parser) parseConstruct() (*ConstructQuery, error) {
	query := &ConstructQuery{}

	p.skipWhitespace()

	// Check for CONSTRUCT WHERE shorthand syntax
	if p.matchKeyword("WHERE") {
		// CONSTRUCT WHERE { pattern } is shorthand for CONSTRUCT { pattern } WHERE { pattern }
		// BUT only when the pattern contains only triple patterns (no FILTER)
		where, err := p.parseGraphPattern()
		if err != nil {
			return nil, err
		}

		// CONSTRUCT WHERE is only valid when there are no FILTER expressions
		// GRAPH patterns and other constructs are allowed
		if len(where.Filters) > 0 {
			return nil, fmt.Errorf("CONSTRUCT WHERE cannot contain FILTER expressions")
		}

		query.Where = where

		// Use the WHERE pattern as the template
		query.Template = where.Patterns

		return query, nil
	}

	// Parse template - expects { triple pattern ... }
	if p.peek() != '{' {
		return nil, fmt.Errorf("expected '{' to start CONSTRUCT template or WHERE keyword")
	}
	p.advance() // skip '{'

	// Parse triple patterns for the template
	var template []*TriplePattern
	for {
		p.skipWhitespace()
		if p.peek() == '}' {
			p.advance() // skip '}'
			break
		}

		// Parse a triple pattern
		pattern, err := p.parseTriplePattern()
		if err != nil {
			return nil, err
		}
		template = append(template, pattern)

		p.skipWhitespace()
		// Optionally consume '.' separator
		if p.peek() == '.' {
			p.advance()
		}
	}

	query.Template = template

	// Parse WHERE clause
	if !p.matchKeyword("WHERE") {
		return nil, fmt.Errorf("expected WHERE clause")
	}

	where, err := p.parseGraphPattern()
	if err != nil {
		return nil, err
	}
	query.Where = where

	return query, nil
}

// parseProjection parses the projection (variables or *)
func (p *Parser) parseProjection() ([]*Variable, error) {
	p.skipWhitespace()

	if p.peek() == '*' {
		p.advance()
		return nil, nil // nil means SELECT *
	}

	var variables []*Variable
	for {
		p.skipWhitespace()
		if p.peek() != '?' && p.peek() != '$' {
			break
		}

		variable, err := p.parseVariable()
		if err != nil {
			return nil, err
		}
		variables = append(variables, variable)
	}

	if len(variables) == 0 {
		return nil, fmt.Errorf("expected at least one variable or *")
	}

	return variables, nil
}

// parseGraphPattern parses a graph pattern (WHERE clause content)
func (p *Parser) parseGraphPattern() (*GraphPattern, error) {
	p.skipWhitespace()

	if p.peek() != '{' {
		return nil, fmt.Errorf("expected '{' to start graph pattern")
	}
	p.advance() // consume '{'

	pattern := &GraphPattern{
		Type:     GraphPatternTypeBasic,
		Patterns: []*TriplePattern{},
		Filters:  []*Filter{},
		Binds:    []*Bind{},
	}

	for {
		p.skipWhitespace()

		// Check for end of pattern
		if p.peek() == '}' {
			p.advance()
			break
		}

		// Check for GRAPH keyword
		if p.matchKeyword("GRAPH") {
			graphPattern, err := p.parseGraphGraphPattern()
			if err != nil {
				return nil, err
			}
			// Add the GRAPH pattern as a child
			if pattern.Children == nil {
				pattern.Children = []*GraphPattern{}
			}
			pattern.Children = append(pattern.Children, graphPattern)
			continue
		}

		// Check for FILTER
		if p.matchKeyword("FILTER") {
			filter, err := p.parseFilter()
			if err != nil {
				return nil, err
			}
			pattern.Filters = append(pattern.Filters, filter)
			continue
		}

		// Check for BIND
		if p.matchKeyword("BIND") {
			bind, err := p.parseBind()
			if err != nil {
				return nil, err
			}
			pattern.Binds = append(pattern.Binds, bind)
			continue
		}

		// Parse triple pattern
		triple, err := p.parseTriplePattern()
		if err != nil {
			return nil, err
		}
		pattern.Patterns = append(pattern.Patterns, triple)

		// Skip optional '.' separator
		p.skipWhitespace()
		if p.peek() == '.' {
			p.advance()
		}
	}

	return pattern, nil
}

// parseGraphGraphPattern parses a GRAPH <iri> { ... } or GRAPH ?var { ... } pattern
func (p *Parser) parseGraphGraphPattern() (*GraphPattern, error) {
	p.skipWhitespace()

	// Parse graph name (IRI or variable)
	graphTerm := &GraphTerm{}

	if p.peek() == '?' {
		// Variable
		varName, err := p.parseVariable()
		if err != nil {
			return nil, err
		}
		graphTerm.Variable = varName
	} else if p.peek() == '<' {
		// IRI
		iri, err := p.parseIRI()
		if err != nil {
			return nil, err
		}
		graphTerm.IRI = rdf.NewNamedNode(iri)
	} else {
		return nil, fmt.Errorf("expected IRI or variable after GRAPH")
	}

	// Parse the nested graph pattern
	nestedPattern, err := p.parseGraphPattern()
	if err != nil {
		return nil, err
	}

	// Create a GRAPH pattern
	graphPattern := &GraphPattern{
		Type:     GraphPatternTypeGraph,
		Graph:    graphTerm,
		Patterns: nestedPattern.Patterns,
		Filters:  nestedPattern.Filters,
		Children: nestedPattern.Children,
	}

	return graphPattern, nil
}

// parseTriplePattern parses a single triple pattern
func (p *Parser) parseTriplePattern() (*TriplePattern, error) {
	p.skipWhitespace()

	subject, err := p.parseTermOrVariable()
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject: %w", err)
	}

	p.skipWhitespace()
	predicate, err := p.parseTermOrVariable()
	if err != nil {
		return nil, fmt.Errorf("failed to parse predicate: %w", err)
	}

	p.skipWhitespace()
	object, err := p.parseTermOrVariable()
	if err != nil {
		return nil, fmt.Errorf("failed to parse object: %w", err)
	}

	return &TriplePattern{
		Subject:   *subject,
		Predicate: *predicate,
		Object:    *object,
	}, nil
}

// parseTermOrVariable parses either an RDF term or a variable
func (p *Parser) parseTermOrVariable() (*TermOrVariable, error) {
	p.skipWhitespace()

	ch := p.peek()

	// Variable
	if ch == '?' || ch == '$' {
		variable, err := p.parseVariable()
		if err != nil {
			return nil, err
		}
		return &TermOrVariable{Variable: variable}, nil
	}

	// IRI (named node)
	if ch == '<' {
		iri, err := p.parseIRI()
		if err != nil {
			return nil, err
		}
		return &TermOrVariable{Term: rdf.NewNamedNode(iri)}, nil
	}

	// Literal (string)
	if ch == '"' || ch == '\'' {
		literal, err := p.parseStringLiteral()
		if err != nil {
			return nil, err
		}
		return &TermOrVariable{Term: literal}, nil
	}

	// Blank node
	if ch == '_' {
		blankNode, err := p.parseBlankNode()
		if err != nil {
			return nil, err
		}
		return &TermOrVariable{Term: blankNode}, nil
	}

	// Numeric literal
	if ch >= '0' && ch <= '9' || ch == '-' || ch == '+' {
		literal, err := p.parseNumericLiteral()
		if err != nil {
			return nil, err
		}
		return &TermOrVariable{Term: literal}, nil
	}

	// Keyword 'a' (shorthand for rdf:type)
	if ch == 'a' {
		// Check if it's just 'a' by itself (not part of a prefixed name)
		if p.pos+1 >= p.length || !((p.input[p.pos+1] >= 'a' && p.input[p.pos+1] <= 'z') ||
			(p.input[p.pos+1] >= 'A' && p.input[p.pos+1] <= 'Z') ||
			(p.input[p.pos+1] >= '0' && p.input[p.pos+1] <= '9') ||
			p.input[p.pos+1] == '_' || p.input[p.pos+1] == '-' || p.input[p.pos+1] == ':') {
			p.advance() // consume 'a'
			return &TermOrVariable{Term: rdf.NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")}, nil
		}
	}

	// Prefixed name (like :foo or prefix:foo)
	if ch == ':' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
		prefixedName, err := p.parsePrefixedName()
		if err != nil {
			return nil, err
		}
		return &TermOrVariable{Term: rdf.NewNamedNode(prefixedName)}, nil
	}

	return nil, fmt.Errorf("unexpected character: %c", ch)
}

// parseVariable parses a SPARQL variable
func (p *Parser) parseVariable() (*Variable, error) {
	if p.peek() != '?' && p.peek() != '$' {
		return nil, fmt.Errorf("expected variable starting with ? or $")
	}
	p.advance() // consume ? or $

	name := p.readWhile(func(ch byte) bool {
		return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_'
	})

	if name == "" {
		return nil, fmt.Errorf("invalid variable name")
	}

	return &Variable{Name: name}, nil
}

// parseIRI parses an IRI enclosed in < >
func (p *Parser) parseIRI() (string, error) {
	if p.peek() != '<' {
		return "", fmt.Errorf("expected '<' to start IRI")
	}
	p.advance()

	iri := p.readWhile(func(ch byte) bool {
		return ch != '>'
	})

	if p.peek() != '>' {
		return "", fmt.Errorf("expected '>' to end IRI")
	}
	p.advance()

	return iri, nil
}

// parseStringLiteral parses a string literal
func (p *Parser) parseStringLiteral() (*rdf.Literal, error) {
	quote := p.peek()
	if quote != '"' && quote != '\'' {
		return nil, fmt.Errorf("expected quote to start string literal")
	}
	p.advance()

	value := p.readWhile(func(ch byte) bool {
		return ch != quote
	})

	if p.peek() != quote {
		return nil, fmt.Errorf("expected quote to end string literal")
	}
	p.advance()

	return rdf.NewLiteral(value), nil
}

// parseBlankNode parses a blank node
func (p *Parser) parseBlankNode() (*rdf.BlankNode, error) {
	if p.peek() != '_' {
		return nil, fmt.Errorf("expected '_' to start blank node")
	}
	p.advance()

	if p.peek() != ':' {
		return nil, fmt.Errorf("expected ':' after '_' in blank node")
	}
	p.advance()

	id := p.readWhile(func(ch byte) bool {
		return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_'
	})

	return rdf.NewBlankNode(id), nil
}

// parseNumericLiteral parses a numeric literal
func (p *Parser) parseNumericLiteral() (*rdf.Literal, error) {
	numStr := p.readWhile(func(ch byte) bool {
		return (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == '+' || ch == 'e' || ch == 'E'
	})

	// Try to parse as integer
	if !strings.Contains(numStr, ".") && !strings.Contains(numStr, "e") && !strings.Contains(numStr, "E") {
		if _, err := strconv.ParseInt(numStr, 10, 64); err == nil {
			return rdf.NewLiteralWithDatatype(numStr, rdf.XSDInteger), nil
		}
	}

	// Parse as double
	return rdf.NewLiteralWithDatatype(numStr, rdf.XSDDouble), nil
}

// parseFilter parses a FILTER expression
func (p *Parser) parseFilter() (*Filter, error) {
	// Simple implementation - just consume until end of expression
	// Full implementation would parse the expression tree
	p.skipWhitespace()

	if p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' after FILTER")
	}

	// Find matching closing parenthesis
	depth := 0
	for p.pos < p.length {
		if p.peek() == '(' {
			depth++
		} else if p.peek() == ')' {
			depth--
			if depth == 0 {
				p.advance()
				break
			}
		}
		p.advance()
	}

	// TODO: Parse expression properly
	return &Filter{}, nil
}

// parseBind parses a BIND expression: BIND(<expression> AS ?variable)
func (p *Parser) parseBind() (*Bind, error) {
	p.skipWhitespace()

	if p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' after BIND")
	}
	p.advance() // skip '('

	// Parse expression - for now, skip until 'AS' keyword
	// Find 'AS' keyword
	startPos := p.pos
	depth := 1
	for p.pos < p.length {
		if p.peek() == '(' {
			depth++
			p.advance()
		} else if p.peek() == ')' {
			depth--
			if depth == 0 {
				// Found closing paren without AS - error
				return nil, fmt.Errorf("expected AS keyword in BIND expression")
			}
			p.advance()
		} else if depth == 1 && p.matchKeyword("AS") {
			// Found AS at the right depth
			break
		} else {
			p.advance()
		}
	}

	// Save the expression text (we'll just store it as a placeholder for now)
	_ = p.input[startPos : p.pos-2] // Skip the " AS" we just consumed

	p.skipWhitespace()

	// Parse variable
	variable, err := p.parseVariable()
	if err != nil {
		return nil, fmt.Errorf("expected variable after AS in BIND: %w", err)
	}

	p.skipWhitespace()

	// Expect closing parenthesis
	if p.peek() != ')' {
		return nil, fmt.Errorf("expected ')' to close BIND expression")
	}
	p.advance() // skip ')'

	// TODO: Parse expression properly
	return &Bind{Variable: variable}, nil
}

// parseOrderBy parses ORDER BY clause
func (p *Parser) parseOrderBy() ([]*OrderCondition, error) {
	// Simplified implementation
	var conditions []*OrderCondition

	for {
		p.skipWhitespace()

		ascending := true
		if p.matchKeyword("DESC") {
			ascending = false
		} else if p.matchKeyword("ASC") {
			ascending = true
		}

		p.skipWhitespace()
		if p.peek() != '?' && p.peek() != '$' {
			break
		}

		variable, err := p.parseVariable()
		if err != nil {
			return nil, err
		}

		conditions = append(conditions, &OrderCondition{
			Expression: &VariableExpression{Variable: variable},
			Ascending:  ascending,
		})

		// Check if there are more conditions
		p.skipWhitespace()
		if !p.matchKeyword("LIMIT") && !p.matchKeyword("OFFSET") && p.pos < p.length {
			continue
		}
		break
	}

	return conditions, nil
}

// parseInteger parses an integer
func (p *Parser) parseInteger() (int, error) {
	p.skipWhitespace()

	numStr := p.readWhile(func(ch byte) bool {
		return ch >= '0' && ch <= '9'
	})

	if numStr == "" {
		return 0, fmt.Errorf("expected integer")
	}

	return strconv.Atoi(numStr)
}

// Helper methods

func (p *Parser) peek() byte {
	if p.pos >= p.length {
		return 0
	}
	return p.input[p.pos]
}

func (p *Parser) advance() {
	if p.pos < p.length {
		p.pos++
	}
}

func (p *Parser) skipWhitespace() {
	for p.pos < p.length {
		ch := p.input[p.pos]

		// Skip whitespace characters
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
			continue
		}

		// Skip comments (from # to end of line)
		if ch == '#' {
			p.pos++
			// Skip until newline
			for p.pos < p.length && p.input[p.pos] != '\n' && p.input[p.pos] != '\r' {
				p.pos++
			}
			continue
		}

		break
	}
}

func (p *Parser) readWhile(predicate func(byte) bool) string {
	start := p.pos
	for p.pos < p.length && predicate(p.input[p.pos]) {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *Parser) matchKeyword(keyword string) bool {
	p.skipWhitespace()

	// Case-insensitive match
	remaining := p.input[p.pos:]
	pattern := `(?i)^` + regexp.QuoteMeta(keyword) + `\b`
	matched, _ := regexp.MatchString(pattern, remaining)

	if matched {
		p.pos += len(keyword)
		return true
	}
	return false
}

// skipPrefix parses and stores a PREFIX declaration (prefix: <iri>)
func (p *Parser) skipPrefix() error {
	p.skipWhitespace()

	// Read prefix name (can be empty for default prefix)
	prefixStart := p.pos
	for p.pos < p.length && p.input[p.pos] != ':' {
		p.advance()
	}
	prefix := p.input[prefixStart:p.pos]

	if p.pos >= p.length {
		return fmt.Errorf("expected ':' in PREFIX declaration")
	}
	p.advance() // skip ':'

	p.skipWhitespace()

	// Parse IRI <...>
	if p.peek() != '<' {
		return fmt.Errorf("expected '<' to start IRI in PREFIX declaration")
	}
	p.advance() // skip '<'

	iriStart := p.pos
	for p.pos < p.length && p.input[p.pos] != '>' {
		p.advance()
	}
	iri := p.input[iriStart:p.pos]

	if p.pos >= p.length {
		return fmt.Errorf("expected '>' to end IRI in PREFIX declaration")
	}
	p.advance() // skip '>'

	// Store the prefix mapping
	p.prefixes[prefix] = iri

	return nil
}

// skipBase skips a BASE declaration (<iri>)
func (p *Parser) skipBase() error {
	p.skipWhitespace()

	// Skip IRI <...>
	if p.peek() != '<' {
		return fmt.Errorf("expected '<' to start IRI in BASE declaration")
	}
	p.advance() // skip '<'

	// Skip until '>'
	for p.pos < p.length && p.input[p.pos] != '>' {
		p.advance()
	}

	if p.pos >= p.length {
		return fmt.Errorf("expected '>' to end IRI in BASE declaration")
	}
	p.advance() // skip '>'

	return nil
}

// parsePrefixedName parses a prefixed name (like :foo or prefix:foo) and expands it to a full IRI
func (p *Parser) parsePrefixedName() (string, error) {
	// Read prefix part (everything before ':')
	prefixStart := p.pos
	for p.pos < p.length && p.input[p.pos] != ':' {
		ch := p.input[p.pos]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			break
		}
		p.advance()
	}
	prefix := p.input[prefixStart:p.pos]

	// Expect ':'
	if p.peek() != ':' {
		return "", fmt.Errorf("expected ':' in prefixed name")
	}
	p.advance() // skip ':'

	// Read local part (everything after ':')
	localStart := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
			break
		}
		p.advance()
	}
	local := p.input[localStart:p.pos]

	// Look up prefix in prefix map
	baseIRI, ok := p.prefixes[prefix]
	if !ok {
		return "", fmt.Errorf("undefined prefix: '%s'", prefix)
	}

	// Expand to full IRI
	return baseIRI + local, nil
}
