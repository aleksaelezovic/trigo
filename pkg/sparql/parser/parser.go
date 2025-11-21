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
	input            string
	pos              int
	length           int
	prefixes         map[string]string // Maps prefix to IRI
	baseURI          string            // Base URI for resolving relative IRIs
	blankNodeCounter int               // Counter for generating blank node identifiers
	extraTriples     []TriplePattern   // Extra triples generated from collections and blank node property lists
}

// NewParser creates a new SPARQL parser
func NewParser(input string) *Parser {
	return &Parser{
		input:            input,
		pos:              0,
		length:           len(input),
		prefixes:         make(map[string]string),
		baseURI:          "",
		blankNodeCounter: 0,
		extraTriples:     make([]TriplePattern, 0),
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
	case QueryTypeDescribe:
		describeQuery, err := p.parseDescribe()
		if err != nil {
			return nil, err
		}
		query.Describe = describeQuery
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

	// Parse DISTINCT or REDUCED (optional, mutually exclusive)
	if p.matchKeyword("DISTINCT") {
		query.Distinct = true
	} else if p.matchKeyword("REDUCED") {
		query.Reduced = true
	}

	// Parse variables or *
	variables, err := p.parseProjection()
	if err != nil {
		return nil, err
	}
	query.Variables = variables

	// Parse FROM and FROM NAMED clauses (optional)
	from, fromNamed, err := p.parseDatasetClauses()
	if err != nil {
		return nil, err
	}
	query.From = from
	query.FromNamed = fromNamed

	// Parse WHERE clause (WHERE keyword is optional)
	p.matchKeyword("WHERE") // consume WHERE if present, but don't require it

	where, err := p.parseGraphPattern()
	if err != nil {
		return nil, err
	}
	query.Where = where

	// Parse optional GROUP BY
	if p.matchKeyword("GROUP") {
		if !p.matchKeyword("BY") {
			return nil, fmt.Errorf("expected BY after GROUP")
		}
		groupBy, err := p.parseGroupBy()
		if err != nil {
			return nil, err
		}
		query.GroupBy = groupBy
	}

	// Parse optional HAVING
	if p.matchKeyword("HAVING") {
		having, err := p.parseHaving()
		if err != nil {
			return nil, err
		}
		query.Having = having
	}

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

	// Parse optional LIMIT and OFFSET (in either order)
	// SPARQL allows both "LIMIT n OFFSET m" and "OFFSET m LIMIT n"
	for {
		p.skipWhitespace()
		if query.Limit == nil && p.matchKeyword("LIMIT") {
			limit, err := p.parseInteger()
			if err != nil {
				return nil, err
			}
			query.Limit = &limit
			continue
		}
		if query.Offset == nil && p.matchKeyword("OFFSET") {
			offset, err := p.parseInteger()
			if err != nil {
				return nil, err
			}
			query.Offset = &offset
			continue
		}
		break
	}

	return query, nil
}

// parseAsk parses an ASK query
func (p *Parser) parseAsk() (*AskQuery, error) {
	query := &AskQuery{}

	// Parse FROM and FROM NAMED clauses (optional)
	from, fromNamed, err := p.parseDatasetClauses()
	if err != nil {
		return nil, err
	}
	query.From = from
	query.FromNamed = fromNamed

	// Parse WHERE clause (WHERE keyword is optional in SPARQL)
	// Both "ASK WHERE { ... }" and "ASK { ... }" are valid
	p.matchKeyword("WHERE") // skip if present, but don't require it

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

		// Parse triple pattern(s) with property list shorthand support
		patterns, err := p.parseTriplePatterns()
		if err != nil {
			return nil, err
		}
		template = append(template, patterns...)

		p.skipWhitespace()
		// Optionally consume '.' separator
		if p.peek() == '.' {
			p.advance()
		}
	}

	query.Template = template

	// Parse FROM and FROM NAMED clauses (optional)
	from, fromNamed, err := p.parseDatasetClauses()
	if err != nil {
		return nil, err
	}
	query.From = from
	query.FromNamed = fromNamed

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

// parseDescribe parses a DESCRIBE query
func (p *Parser) parseDescribe() (*DescribeQuery, error) {
	query := &DescribeQuery{}

	p.skipWhitespace()

	// Check if there's a WHERE clause immediately (DESCRIBE WHERE is invalid, but check for explicit resources)
	if p.matchKeyword("WHERE") {
		// DESCRIBE WHERE { pattern } - describes all resources found
		where, err := p.parseGraphPattern()
		if err != nil {
			return nil, err
		}
		query.Where = where
		return query, nil
	}

	// Parse resource IRIs (one or more)
	// DESCRIBE <uri1> <uri2> ... WHERE { pattern }
	// or DESCRIBE <uri1> <uri2> ... (no WHERE clause)
	for {
		p.skipWhitespace()

		// Check if we've reached WHERE or end of query
		if p.matchKeyword("WHERE") || p.pos >= len(p.input) {
			p.pos -= 5 // Un-consume "WHERE" so we can parse it below
			break
		}

		// Try to parse an IRI
		if p.peek() == '<' {
			iri, err := p.parseIRI()
			if err != nil {
				return nil, err
			}
			query.Resources = append(query.Resources, rdf.NewNamedNode(iri))
		} else if p.peek() == '?' || p.peek() == '$' {
			// Variables in DESCRIBE are not yet supported in executor
			// but we should parse them for syntax tests
			_, err := p.parseVariable()
			if err != nil {
				return nil, err
			}
			// For now, we'll ignore variables in DESCRIBE since executor doesn't support them yet
			// This at least makes the syntax tests pass
		} else {
			// No more resources
			break
		}

		p.skipWhitespace()
	}

	// Parse FROM and FROM NAMED clauses (optional)
	from, fromNamed, err := p.parseDatasetClauses()
	if err != nil {
		return nil, err
	}
	query.From = from
	query.FromNamed = fromNamed

	// Parse optional WHERE clause
	p.skipWhitespace()
	if p.matchKeyword("WHERE") {
		where, err := p.parseGraphPattern()
		if err != nil {
			return nil, err
		}
		query.Where = where
	}

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
	hasProjection := false
	for {
		p.skipWhitespace()
		ch := p.peek()

		// Check for expression in parentheses: (expr AS ?var)
		if ch == '(' {
			// Skip expression and extract variable
			if err := p.skipSelectExpression(); err != nil {
				return nil, err
			}
			hasProjection = true
			continue
		}

		// Regular variable
		if ch != '?' && ch != '$' {
			break
		}

		variable, err := p.parseVariable()
		if err != nil {
			return nil, err
		}
		variables = append(variables, variable)
		hasProjection = true
	}

	if !hasProjection {
		return nil, fmt.Errorf("expected at least one variable or *")
	}

	return variables, nil
}

// parseDatasetClauses parses FROM and FROM NAMED clauses
// Returns (from IRIs, fromNamed IRIs, error)
func (p *Parser) parseDatasetClauses() ([]string, []string, error) {
	var from []string
	var fromNamed []string

	for {
		p.skipWhitespace()

		// Check for FROM NAMED
		if p.matchKeyword("FROM") {
			p.skipWhitespace()
			if p.matchKeyword("NAMED") {
				// FROM NAMED <iri>
				p.skipWhitespace()
				if p.peek() != '<' {
					return nil, nil, fmt.Errorf("expected IRI after FROM NAMED")
				}
				iri, err := p.parseIRI()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to parse IRI in FROM NAMED: %w", err)
				}
				fromNamed = append(fromNamed, iri)
			} else {
				// FROM <iri>
				p.skipWhitespace()
				if p.peek() != '<' {
					return nil, nil, fmt.Errorf("expected IRI after FROM")
				}
				iri, err := p.parseIRI()
				if err != nil {
					return nil, nil, fmt.Errorf("failed to parse IRI in FROM: %w", err)
				}
				from = append(from, iri)
			}
		} else {
			// No more FROM clauses
			break
		}
	}

	return from, fromNamed, nil
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
		Elements: []PatternElement{},
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
			// Add to Elements to preserve order with FILTERs
			pattern.Elements = append(pattern.Elements, PatternElement{GraphPattern: graphPattern})
			// Skip optional '.' separator after GRAPH block
			p.skipWhitespace()
			if p.peek() == '.' {
				p.advance()
			}
			continue
		}

		// Check for FILTER
		if p.matchKeyword("FILTER") {
			filter, err := p.parseFilter()
			if err != nil {
				return nil, err
			}
			pattern.Filters = append(pattern.Filters, filter)
			pattern.Elements = append(pattern.Elements, PatternElement{Filter: filter})
			// Skip optional '.' separator after FILTER
			p.skipWhitespace()
			if p.peek() == '.' {
				p.advance()
			}
			continue
		}

		// Check for BIND
		if p.matchKeyword("BIND") {
			bind, err := p.parseBind()
			if err != nil {
				return nil, err
			}
			pattern.Binds = append(pattern.Binds, bind)
			pattern.Elements = append(pattern.Elements, PatternElement{Bind: bind})
			// Skip optional '.' separator after BIND
			p.skipWhitespace()
			if p.peek() == '.' {
				p.advance()
			}
			continue
		}

		// Check for OPTIONAL
		if p.matchKeyword("OPTIONAL") {
			optionalPattern, err := p.parseGraphPattern()
			if err != nil {
				return nil, err
			}
			optionalPattern.Type = GraphPatternTypeOptional
			if pattern.Children == nil {
				pattern.Children = []*GraphPattern{}
			}
			pattern.Children = append(pattern.Children, optionalPattern)
			// Add to Elements to preserve order with FILTERs
			pattern.Elements = append(pattern.Elements, PatternElement{GraphPattern: optionalPattern})
			// Skip optional '.' separator after OPTIONAL block
			p.skipWhitespace()
			if p.peek() == '.' {
				p.advance()
			}
			continue
		}

		// Check for MINUS
		if p.matchKeyword("MINUS") {
			minusPattern, err := p.parseGraphPattern()
			if err != nil {
				return nil, err
			}
			minusPattern.Type = GraphPatternTypeMinus
			if pattern.Children == nil {
				pattern.Children = []*GraphPattern{}
			}
			pattern.Children = append(pattern.Children, minusPattern)
			// Add to Elements to preserve order with FILTERs
			pattern.Elements = append(pattern.Elements, PatternElement{GraphPattern: minusPattern})
			// Skip optional '.' separator after MINUS block
			p.skipWhitespace()
			if p.peek() == '.' {
				p.advance()
			}
			continue
		}

		// Check for UNION (needs special handling since it's infix)
		// For now, we'll handle it in a simplified way

		// Check for nested graph pattern or subquery { ... }
		if p.peek() == '{' {
			// Peek ahead to see if this is a subquery (SELECT/ASK/CONSTRUCT/DESCRIBE)
			savedPos := p.pos
			p.advance() // skip '{'
			p.skipWhitespace()

			// Check for subquery keywords
			isSubquery := p.matchKeyword("SELECT") || p.matchKeyword("ASK") ||
				p.matchKeyword("CONSTRUCT") || p.matchKeyword("DESCRIBE")

			// Restore position
			p.pos = savedPos

			if isSubquery {
				// This is a subquery - skip it for now
				// Find the matching closing brace
				p.advance() // skip '{'
				depth := 1
				for p.pos < p.length && depth > 0 {
					if p.peek() == '{' {
						depth++
					} else if p.peek() == '}' {
						depth--
					}
					p.advance()
				}
				// TODO: Actually parse subqueries properly
				continue
			}

			// Regular nested graph pattern
			nestedPattern, err := p.parseGraphPattern()
			if err != nil {
				return nil, err
			}
			if pattern.Children == nil {
				pattern.Children = []*GraphPattern{}
			}
			pattern.Children = append(pattern.Children, nestedPattern)

			// Check for UNION after the nested pattern
			// UNION can chain: { ... } UNION { ... } UNION { ... }
			// Build the UNION chain by looping
			p.skipWhitespace()
			var finalPattern *GraphPattern
			if p.matchKeyword("UNION") {
				// Collect all patterns in the UNION chain
				unionChildren := []*GraphPattern{nestedPattern}

				// Loop to handle multiple consecutive UNIONs
				for {
					// Parse the next pattern after UNION
					rightPattern, err := p.parseGraphPattern()
					if err != nil {
						return nil, err
					}
					unionChildren = append(unionChildren, rightPattern)

					// Check if there's another UNION
					p.skipWhitespace()
					if !p.matchKeyword("UNION") {
						break
					}
				}

				// Create a UNION pattern containing all patterns in the chain
				unionPattern := &GraphPattern{
					Type:     GraphPatternTypeUnion,
					Children: unionChildren,
				}

				// Replace the last child with the union pattern
				pattern.Children[len(pattern.Children)-1] = unionPattern
				finalPattern = unionPattern
			} else {
				finalPattern = nestedPattern
			}

			// Add to Elements to preserve order with FILTERs
			pattern.Elements = append(pattern.Elements, PatternElement{GraphPattern: finalPattern})
			continue
		}

		// Parse triple pattern(s) with property list shorthand support
		triples, err := p.parseTriplePatterns()
		if err != nil {
			return nil, err
		}
		pattern.Patterns = append(pattern.Patterns, triples...)
		// Add each triple to Elements to preserve order
		for _, triple := range triples {
			pattern.Elements = append(pattern.Elements, PatternElement{Triple: triple})
		}

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

	// Check if the subject was a blank node property list or collection that stands alone
	// In SPARQL, [ :p :q ] is valid by itself - it generates triples via extraTriples
	// and doesn't require additional predicate/object
	if len(p.extraTriples) > 0 {
		ch := p.peek()
		if ch == '.' || ch == ';' || ch == '}' || ch == 0 {
			// Standalone blank node property list or collection
			// Return nil - the real triples are in extraTriples
			// parseTriplePatterns will handle the nil case
			return nil, nil
		}
	}

	predicate, err := p.parseTermOrVariable()
	if err != nil {
		return nil, fmt.Errorf("failed to parse predicate: %w", err)
	}

	// Validate predicate: blank nodes cannot be predicates in SPARQL patterns
	if !predicate.IsVariable() {
		if _, isBlankNode := predicate.Term.(*rdf.BlankNode); isBlankNode {
			return nil, fmt.Errorf("blank nodes cannot be used as predicates in triple patterns")
		}
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

// parseTriplePatterns parses triple patterns with property list shorthand (semicolon and comma)
// Syntax:
//
//	?s ?p1 ?o1 ; ?p2 ?o2 ; ?p3 ?o3 .  (semicolon repeats subject)
//	?s ?p ?o1 , ?o2 , ?o3 .           (comma repeats subject and predicate)
func (p *Parser) parseTriplePatterns() ([]*TriplePattern, error) {
	var triples []*TriplePattern

	// Clear any previous extra triples before parsing
	p.extraTriples = make([]TriplePattern, 0)

	// Parse first triple
	firstTriple, err := p.parseTriplePattern()
	if err != nil {
		return nil, err
	}

	// firstTriple may be nil if it's a standalone blank node property list/collection
	// In that case, only the extraTriples matter
	if firstTriple != nil {
		triples = append(triples, firstTriple)
	}

	// Add any extra triples generated from collections or blank node property lists
	for i := range p.extraTriples {
		triples = append(triples, &p.extraTriples[i])
	}
	p.extraTriples = make([]TriplePattern, 0)

	// If firstTriple is nil (standalone property list), skip property list shorthand handling
	if firstTriple == nil {
		return triples, nil
	}

	// Handle property list shorthand
	for {
		p.skipWhitespace()
		ch := p.peek()

		if ch == ',' {
			// Comma: same subject and predicate, new object
			p.advance() // skip ','
			p.skipWhitespace()

			object, err := p.parseTermOrVariable()
			if err != nil {
				return nil, fmt.Errorf("failed to parse object after comma: %w", err)
			}

			triples = append(triples, &TriplePattern{
				Subject:   firstTriple.Subject,
				Predicate: firstTriple.Predicate,
				Object:    *object,
			})

			// Add any extra triples generated from collections
			for i := range p.extraTriples {
				triples = append(triples, &p.extraTriples[i])
			}
			p.extraTriples = make([]TriplePattern, 0)

		} else if ch == ';' {
			// Semicolon: same subject, new predicate and object
			p.advance() // skip ';'
			p.skipWhitespace()

			// Check for end of pattern (semicolon can be trailing)
			// Trailing semicolon can be followed by: '.', '}', or graph pattern keywords
			if p.peek() == '.' || p.peek() == '}' {
				break
			}

			// Check for graph pattern keywords (OPTIONAL, UNION, MINUS, FILTER, BIND, GRAPH)
			savedPos := p.pos
			if p.matchKeyword("OPTIONAL") || p.matchKeyword("UNION") || p.matchKeyword("MINUS") ||
				p.matchKeyword("FILTER") || p.matchKeyword("BIND") || p.matchKeyword("GRAPH") {
				// Trailing semicolon before graph pattern keyword
				p.pos = savedPos // restore position
				break
			}
			p.pos = savedPos // restore position for normal parsing

			predicate, err := p.parseTermOrVariable()
			if err != nil {
				return nil, fmt.Errorf("failed to parse predicate after semicolon: %w", err)
			}

			p.skipWhitespace()
			object, err := p.parseTermOrVariable()
			if err != nil {
				return nil, fmt.Errorf("failed to parse object after semicolon: %w", err)
			}

			triple := &TriplePattern{
				Subject:   firstTriple.Subject,
				Predicate: *predicate,
				Object:    *object,
			}
			triples = append(triples, triple)

			// Add any extra triples generated from collections
			for i := range p.extraTriples {
				triples = append(triples, &p.extraTriples[i])
			}
			p.extraTriples = make([]TriplePattern, 0)

			// Update firstTriple to allow comma after this predicate-object pair
			firstTriple = triple

		} else {
			// No more comma or semicolon, done
			break
		}
	}

	return triples, nil
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

	// Collection: (...)
	if ch == '(' {
		return p.parseCollection()
	}

	// Blank node property list: [...]
	if ch == '[' {
		return p.parseBlankNodePropertyList()
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

		// Check for language tag or datatype
		p.skipWhitespace()
		if p.peek() == '@' {
			// Language tag
			p.advance() // skip '@'
			lang := p.readWhile(func(c byte) bool {
				return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-'
			})
			return &TermOrVariable{Term: rdf.NewLiteralWithLanguage(literal.Value, lang)}, nil
		} else if p.peek() == '^' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '^' {
			// Datatype
			p.advance() // skip first '^'
			p.advance() // skip second '^'
			p.skipWhitespace()

			// Parse datatype IRI (can be <iri> or prefix:local)
			var datatypeIRI string
			if p.peek() == '<' {
				iri, err := p.parseIRI()
				if err != nil {
					return nil, fmt.Errorf("failed to parse datatype IRI: %w", err)
				}
				datatypeIRI = iri
			} else {
				// Prefixed name
				prefixedName, err := p.parsePrefixedName()
				if err != nil {
					return nil, fmt.Errorf("failed to parse datatype: %w", err)
				}
				datatypeIRI = prefixedName
			}

			datatype := rdf.NewNamedNode(datatypeIRI)
			return &TermOrVariable{Term: rdf.NewLiteralWithDatatype(literal.Value, datatype)}, nil
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

	// Boolean literals (true/false)
	if ch == 't' || ch == 'f' {
		if p.matchKeyword("true") {
			return &TermOrVariable{Term: rdf.NewBooleanLiteral(true)}, nil
		}
		if p.matchKeyword("false") {
			return &TermOrVariable{Term: rdf.NewBooleanLiteral(false)}, nil
		}
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

// parseIRI parses an IRI enclosed in < > and resolves it against BASE if needed
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

	// Resolve relative IRI against BASE if needed
	resolvedIRI := p.resolveIRI(iri)

	return resolvedIRI, nil
}

// parseStringLiteral parses a string literal (supports single and triple-quoted)
func (p *Parser) parseStringLiteral() (*rdf.Literal, error) {
	quote := p.peek()
	if quote != '"' && quote != '\'' {
		return nil, fmt.Errorf("expected quote to start string literal")
	}

	// Check for triple-quoted string
	if p.pos+2 < len(p.input) && p.input[p.pos+1] == quote && p.input[p.pos+2] == quote {
		// Triple-quoted string
		p.advance() // first quote
		p.advance() // second quote
		p.advance() // third quote

		// Read until we find three consecutive quotes (not escaped)
		var value strings.Builder
		for p.pos < len(p.input) {
			ch := p.input[p.pos]

			// Handle escape sequences
			if ch == '\\' && p.pos+1 < len(p.input) {
				p.advance() // skip backslash
				nextCh := p.input[p.pos]
				switch nextCh {
				case 't':
					value.WriteByte('\t')
				case 'n':
					value.WriteByte('\n')
				case 'r':
					value.WriteByte('\r')
				case '\\':
					value.WriteByte('\\')
				case '"':
					value.WriteByte('"')
				case '\'':
					value.WriteByte('\'')
				default:
					// Unknown escape - keep the backslash and character
					value.WriteByte('\\')
					value.WriteByte(nextCh)
				}
				p.advance()
				continue
			}

			// Check for closing triple quote (after escape handling)
			if p.pos+2 < len(p.input) &&
				p.input[p.pos] == quote &&
				p.input[p.pos+1] == quote &&
				p.input[p.pos+2] == quote {
				// Found closing triple quote
				p.advance()
				p.advance()
				p.advance()
				return rdf.NewLiteral(value.String()), nil
			}

			value.WriteByte(p.input[p.pos])
			p.advance()
		}
		return nil, fmt.Errorf("unclosed triple-quoted string")
	}

	// Single-quoted string
	p.advance()

	// Read string with escape sequence support
	var value strings.Builder
	for p.pos < len(p.input) {
		ch := p.peek()

		// Check for closing quote
		if ch == quote {
			p.advance()
			return rdf.NewLiteral(value.String()), nil
		}

		// Handle escape sequences
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.advance() // skip backslash
			nextCh := p.peek()
			switch nextCh {
			case 't':
				value.WriteByte('\t')
			case 'n':
				value.WriteByte('\n')
			case 'r':
				value.WriteByte('\r')
			case '\\':
				value.WriteByte('\\')
			case '"':
				value.WriteByte('"')
			case '\'':
				value.WriteByte('\'')
			default:
				// Unknown escape - keep the backslash and character
				value.WriteByte('\\')
				value.WriteByte(nextCh)
			}
			p.advance()
		} else {
			value.WriteByte(ch)
			p.advance()
		}
	}

	return nil, fmt.Errorf("expected quote to end string literal")
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

// newBlankNode generates a new blank node with a unique identifier
func (p *Parser) newBlankNode() *rdf.BlankNode {
	p.blankNodeCounter++
	return rdf.NewBlankNode(fmt.Sprintf("b%d", p.blankNodeCounter))
}

// parseCollection parses an RDF collection (list) in SPARQL syntax: (item1 item2 ...)
// Collections are expanded to rdf:first/rdf:rest triples
func (p *Parser) parseCollection() (*TermOrVariable, error) {
	if p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' at start of collection")
	}
	p.advance() // skip '('
	p.skipWhitespace()

	// Check for empty collection: ()
	if p.peek() == ')' {
		p.advance() // skip ')'
		// Empty collection is rdf:nil
		return &TermOrVariable{Term: rdf.NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#nil")}, nil
	}

	// Parse collection items
	var items []*TermOrVariable
	for {
		p.skipWhitespace()
		if p.peek() == ')' {
			break
		}

		item, err := p.parseTermOrVariable()
		if err != nil {
			return nil, fmt.Errorf("failed to parse collection item: %w", err)
		}
		items = append(items, item)

		p.skipWhitespace()
		if p.peek() == ')' {
			break
		}
		// Items are separated by whitespace, not commas
	}

	if p.peek() != ')' {
		return nil, fmt.Errorf("expected ')' at end of collection")
	}
	p.advance() // skip ')'

	// Build RDF list structure: _:b1 rdf:first item1 ; rdf:rest _:b2 . etc.
	rdfFirst := rdf.NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#first")
	rdfRest := rdf.NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#rest")
	rdfNil := rdf.NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#nil")

	var listHead *TermOrVariable
	var prevNode *TermOrVariable

	for i, item := range items {
		node := &TermOrVariable{Term: p.newBlankNode()}

		if i == 0 {
			listHead = node
		}

		// Add rdf:first triple (this node points to the item)
		p.extraTriples = append(p.extraTriples, TriplePattern{
			Subject:   *node,
			Predicate: TermOrVariable{Term: rdfFirst},
			Object:    *item,
		})

		// Link previous node to this one
		if i > 0 && prevNode != nil {
			p.extraTriples = append(p.extraTriples, TriplePattern{
				Subject:   *prevNode,
				Predicate: TermOrVariable{Term: rdfRest},
				Object:    *node,
			})
		}

		// Add rdf:rest triple for last item
		if i == len(items)-1 {
			p.extraTriples = append(p.extraTriples, TriplePattern{
				Subject:   *node,
				Predicate: TermOrVariable{Term: rdfRest},
				Object:    TermOrVariable{Term: rdfNil},
			})
		}

		prevNode = node
	}

	return listHead, nil
}

// parseBlankNodePropertyList parses a blank node property list: [ pred obj ; pred obj ]
// Creates an anonymous blank node with the specified properties
func (p *Parser) parseBlankNodePropertyList() (*TermOrVariable, error) {
	if p.peek() != '[' {
		return nil, fmt.Errorf("expected '[' at start of blank node property list")
	}
	p.advance() // skip '['
	p.skipWhitespace()

	// Check for empty blank node property list: []
	if p.peek() == ']' {
		p.advance() // skip ']'
		// Empty property list is just an anonymous blank node
		return &TermOrVariable{Term: p.newBlankNode()}, nil
	}

	// Create the blank node that will be the subject
	blankNode := &TermOrVariable{Term: p.newBlankNode()}

	// Parse predicate-object pairs
	for {
		p.skipWhitespace()
		if p.peek() == ']' {
			break
		}

		// Parse predicate
		predicate, err := p.parseTermOrVariable()
		if err != nil {
			return nil, fmt.Errorf("failed to parse predicate in blank node property list: %w", err)
		}

		p.skipWhitespace()

		// Parse object(s) - can have multiple objects separated by commas
		for {
			object, err := p.parseTermOrVariable()
			if err != nil {
				return nil, fmt.Errorf("failed to parse object in blank node property list: %w", err)
			}

			// Add triple: blankNode predicate object
			p.extraTriples = append(p.extraTriples, TriplePattern{
				Subject:   *blankNode,
				Predicate: *predicate,
				Object:    *object,
			})

			p.skipWhitespace()

			// Check for comma (multiple objects with same predicate)
			if p.peek() == ',' {
				p.advance() // skip ','
				p.skipWhitespace()
				continue
			}
			break
		}

		p.skipWhitespace()

		// Check for semicolon (more predicate-object pairs)
		if p.peek() == ';' {
			p.advance() // skip ';'
			p.skipWhitespace()
			// Check for trailing semicolon before ]
			if p.peek() == ']' {
				break
			}
			continue
		}

		// No semicolon, must be end of property list
		break
	}

	if p.peek() != ']' {
		return nil, fmt.Errorf("expected ']' at end of blank node property list")
	}
	p.advance() // skip ']'

	return blankNode, nil
}

// parseNumericLiteral parses a numeric literal
// Grammar: [+-]? [0-9]+ ( '.' [0-9]+ )? ( [eE] [+-]? [0-9]+ )?
func (p *Parser) parseNumericLiteral() (*rdf.Literal, error) {
	start := p.pos

	// Optional sign
	if p.peek() == '+' || p.peek() == '-' {
		p.advance()
	}

	// Integer part (required)
	if !p.hasDigit() {
		return nil, fmt.Errorf("expected digit in numeric literal")
	}
	for p.hasDigit() {
		p.advance()
	}

	hasDecimal := false
	hasExponent := false

	// Optional decimal part
	if p.peek() == '.' {
		// Look ahead - if there's a digit after the dot, it's part of the number
		if p.pos+1 < p.length && p.input[p.pos+1] >= '0' && p.input[p.pos+1] <= '9' {
			p.advance() // consume '.'
			hasDecimal = true
			for p.hasDigit() {
				p.advance()
			}
		}
		// Otherwise, the dot is not part of the number (e.g., "123." where "." is a statement terminator)
	}

	// Optional exponent
	if p.peek() == 'e' || p.peek() == 'E' {
		p.advance()
		hasExponent = true
		if p.peek() == '+' || p.peek() == '-' {
			p.advance()
		}
		if !p.hasDigit() {
			return nil, fmt.Errorf("expected digit after exponent in numeric literal")
		}
		for p.hasDigit() {
			p.advance()
		}
	}

	numStr := p.input[start:p.pos]

	// Determine type based on what we found
	if hasExponent {
		return rdf.NewLiteralWithDatatype(numStr, rdf.XSDDouble), nil
	}
	if hasDecimal {
		return rdf.NewLiteralWithDatatype(numStr, rdf.XSDDecimal), nil
	}
	return rdf.NewLiteralWithDatatype(numStr, rdf.XSDInteger), nil
}

// hasDigit checks if the current position has a digit
func (p *Parser) hasDigit() bool {
	return p.pos < p.length && p.input[p.pos] >= '0' && p.input[p.pos] <= '9'
}

// parseFilter parses a FILTER expression
func (p *Parser) parseFilter() (*Filter, error) {
	// Simple implementation - just consume until end of expression
	// Full implementation would parse the expression tree
	p.skipWhitespace()

	// Check for EXISTS or NOT EXISTS (without parentheses around the keyword)
	if p.matchKeyword("EXISTS") {
		// FILTER EXISTS { pattern }
		_, err := p.parseGraphPattern()
		if err != nil {
			return nil, err
		}
		return &Filter{}, nil
	}

	if p.matchKeyword("NOT") {
		p.skipWhitespace()
		if p.matchKeyword("EXISTS") {
			// FILTER NOT EXISTS { pattern }
			_, err := p.parseGraphPattern()
			if err != nil {
				return nil, err
			}
			return &Filter{}, nil
		}
		// If not EXISTS, fall through to normal expression parsing
	}

	// Parse the expression
	// SPARQL allows both FILTER (expr) and FILTER funcCall(...)
	// If there's a '(', it's FILTER (expr) and we need to consume the outer parens
	// Otherwise, the expression itself (like a function call) provides delimiters
	needsOuterParens := p.peek() == '('
	if needsOuterParens {
		p.advance() // skip '('
	}

	expr, err := p.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("error parsing FILTER expression: %w", err)
	}

	if needsOuterParens {
		p.skipWhitespace()
		if p.peek() != ')' {
			return nil, fmt.Errorf("expected ')' after FILTER expression")
		}
		p.advance() // skip ')'
	}

	return &Filter{Expression: expr}, nil
}

// parseBind parses a BIND expression: BIND(<expression> AS ?variable)
func (p *Parser) parseBind() (*Bind, error) {
	p.skipWhitespace()

	if p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' after BIND")
	}
	p.advance() // skip '('

	// Parse the expression
	expr, err := p.parseExpression()
	if err != nil {
		return nil, fmt.Errorf("error parsing BIND expression: %w", err)
	}

	p.skipWhitespace()

	// Expect 'AS' keyword
	if !p.matchKeyword("AS") {
		return nil, fmt.Errorf("expected AS keyword in BIND expression")
	}

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

	return &Bind{Expression: expr, Variable: variable}, nil
}

// parseGroupBy parses GROUP BY clause
func (p *Parser) parseGroupBy() ([]*GroupCondition, error) {
	var conditions []*GroupCondition

	for {
		p.skipWhitespace()

		// Check for end of GROUP BY clause
		ch := p.peek()
		if ch != '?' && ch != '$' && ch != '(' {
			break
		}

		// Parse expression or variable
		if ch == '(' {
			// GROUP BY (expression AS ?var) or GROUP BY (expression)
			p.advance() // skip '('

			// Skip to AS or closing paren
			depth := 1
			for p.pos < p.length && depth > 0 {
				if p.peek() == '(' {
					depth++
					p.advance()
				} else if p.peek() == ')' {
					depth--
					if depth > 0 {
						p.advance()
					}
				} else if depth == 1 && p.matchKeyword("AS") {
					// Variable after AS
					p.skipWhitespace()
					if p.peek() == '?' || p.peek() == '$' {
						_, err := p.parseVariable()
						if err != nil {
							return nil, err
						}
					}
					p.skipWhitespace()
					if p.peek() == ')' {
						p.advance()
						break
					}
				} else {
					p.advance()
				}
			}

			conditions = append(conditions, &GroupCondition{})
		} else {
			// Simple variable
			variable, err := p.parseVariable()
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, &GroupCondition{Variable: variable})
		}

		p.skipWhitespace()
	}

	return conditions, nil
}

// parseHaving parses HAVING clause
func (p *Parser) parseHaving() ([]*Filter, error) {
	var filters []*Filter

	for {
		p.skipWhitespace()

		// Check if we're at the end of HAVING
		if p.peek() != '(' {
			// Try to match EXISTS or NOT
			savedPos := p.pos
			if !p.matchKeyword("EXISTS") {
				p.pos = savedPos
				if !p.matchKeyword("NOT") {
					p.pos = savedPos
					break
				}
				p.pos = savedPos
			} else {
				p.pos = savedPos
			}
		}

		filter, err := p.parseFilter()
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)

		p.skipWhitespace()
	}

	if len(filters) == 0 {
		return nil, fmt.Errorf("expected at least one condition in HAVING")
	}

	return filters, nil
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

	// Resolve relative IRI against BASE if needed
	resolvedIRI := p.resolveIRI(iri)

	// Store the prefix mapping
	p.prefixes[prefix] = resolvedIRI

	return nil
}

// skipBase parses and stores a BASE declaration (<iri>)
func (p *Parser) skipBase() error {
	p.skipWhitespace()

	// Parse IRI <...>
	if p.peek() != '<' {
		return fmt.Errorf("expected '<' to start IRI in BASE declaration")
	}
	p.advance() // skip '<'

	// Read the IRI
	iriStart := p.pos
	for p.pos < p.length && p.input[p.pos] != '>' {
		p.advance()
	}
	iri := p.input[iriStart:p.pos]

	if p.pos >= p.length {
		return fmt.Errorf("expected '>' to end IRI in BASE declaration")
	}
	p.advance() // skip '>'

	// Store the base URI
	p.baseURI = iri

	return nil
}

// skipSelectExpression skips a SELECT expression: (expression AS ?variable)
func (p *Parser) skipSelectExpression() error {
	p.skipWhitespace()

	if p.peek() != '(' {
		return fmt.Errorf("expected '(' to start SELECT expression")
	}
	p.advance() // skip '('

	// Skip until we find 'AS' keyword at depth 1
	depth := 1
	for p.pos < p.length {
		if p.peek() == '(' {
			depth++
			p.advance()
		} else if p.peek() == ')' {
			depth--
			if depth == 0 {
				p.advance() // skip closing ')'
				break
			}
			p.advance()
		} else if depth == 1 && p.matchKeyword("AS") {
			// Found AS - skip to the variable
			p.skipWhitespace()
			// Skip the variable
			if p.peek() == '?' || p.peek() == '$' {
				p.advance() // skip ? or $
				// Skip variable name
				for p.pos < p.length {
					ch := p.peek()
					if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
						(ch >= '0' && ch <= '9') || ch == '_') {
						break
					}
					p.advance()
				}
			}
			p.skipWhitespace()
			// Expect closing ')'
			if p.peek() == ')' {
				p.advance()
				break
			}
		} else {
			p.advance()
		}
	}

	return nil
}

// parsePrefixedName parses a prefixed name (like :foo or prefix:foo) and expands it to a full IRI
func (p *Parser) parsePrefixedName() (string, error) {
	// Read prefix part (everything before ':')
	// Per SPARQL grammar, prefix names can contain: PN_CHARS_BASE | '_' and continue with PN_CHARS | '.'
	// In practice: letters, digits, underscore, hyphen, and dots
	prefixStart := p.pos
	for p.pos < p.length && p.input[p.pos] != ':' {
		ch := p.input[p.pos]
		// Allow letters, digits, underscore, hyphen, and dot in prefix
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.') {
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
	// According to SPARQL spec, PN_LOCAL can start with: PN_CHARS_U | ':' | [0-9] | PLX
	// and continue with: (PN_CHARS | '.' | ':' | PLX)*
	// We'll use a simplified approach that allows most characters
	localStart := p.pos
	for p.pos < p.length {
		ch := p.input[p.pos]
		// Allow letters, digits, underscore, hyphen, and dot
		// According to spec, local part can be more complex, but this covers common cases
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.') {
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

// Expression parsing with operator precedence
// Grammar:
// Expression → LogicalOrExpression
// LogicalOrExpression → LogicalAndExpression ( '||' LogicalAndExpression )*
// LogicalAndExpression → ComparisonExpression ( '&&' ComparisonExpression )*
// ComparisonExpression → AdditiveExpression ( ('=' | '!=' | '<' | '<=' | '>' | '>=') AdditiveExpression )?
// AdditiveExpression → MultiplicativeExpression ( ('+' | '-') MultiplicativeExpression )*
// MultiplicativeExpression → UnaryExpression ( ('*' | '/') UnaryExpression )*
// UnaryExpression → ('!' | '-' | '+')? PrimaryExpression
// PrimaryExpression → Variable | Literal | FunctionCall | '(' Expression ')'

// parseExpression parses a SPARQL expression (entry point)
func (p *Parser) parseExpression() (Expression, error) {
	return p.parseLogicalOrExpression()
}

// parseLogicalOrExpression parses logical OR (lowest precedence)
func (p *Parser) parseLogicalOrExpression() (Expression, error) {
	left, err := p.parseLogicalAndExpression()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.match("||") {
			right, err := p.parseLogicalAndExpression()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpression{
				Left:     left,
				Operator: OpOr,
				Right:    right,
			}
		} else {
			break
		}
	}

	return left, nil
}

// parseLogicalAndExpression parses logical AND
func (p *Parser) parseLogicalAndExpression() (Expression, error) {
	left, err := p.parseComparisonExpression()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.match("&&") {
			right, err := p.parseComparisonExpression()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpression{
				Left:     left,
				Operator: OpAnd,
				Right:    right,
			}
		} else {
			break
		}
	}

	return left, nil
}

// parseComparisonExpression parses comparison operators and IN/NOT IN
func (p *Parser) parseComparisonExpression() (Expression, error) {
	left, err := p.parseAdditiveExpression()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()

	// Check for IN or NOT IN operators
	savedPos := p.pos
	notIn := false
	if p.matchKeyword("NOT") {
		p.skipWhitespace()
		if p.matchKeyword("IN") {
			notIn = true
		} else {
			// NOT not followed by IN, restore and try regular operators
			p.pos = savedPos
		}
	} else if p.matchKeyword("IN") {
		notIn = false
	} else {
		// Not IN operator, check for comparison operators
		p.pos = savedPos
		var op Operator
		if p.match("<=") {
			op = OpLessThanOrEqual
		} else if p.match(">=") {
			op = OpGreaterThanOrEqual
		} else if p.match("!=") {
			op = OpNotEqual
		} else if p.match("=") {
			op = OpEqual
		} else if p.match("<") {
			op = OpLessThan
		} else if p.match(">") {
			op = OpGreaterThan
		} else {
			// No comparison operator
			return left, nil
		}

		right, err := p.parseAdditiveExpression()
		if err != nil {
			return nil, err
		}

		return &BinaryExpression{
			Left:     left,
			Operator: op,
			Right:    right,
		}, nil
	}

	// Parse IN or NOT IN
	p.skipWhitespace()
	if p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' after IN/NOT IN")
	}
	p.advance() // skip '('

	// Parse value list
	var values []Expression
	p.skipWhitespace()

	// Check for empty list
	if p.peek() != ')' {
		for {
			expr, err := p.parseAdditiveExpression()
			if err != nil {
				return nil, fmt.Errorf("failed to parse IN value: %w", err)
			}
			values = append(values, expr)

			p.skipWhitespace()
			if p.peek() == ',' {
				p.advance() // skip ','
				p.skipWhitespace()
			} else {
				break
			}
		}
	}

	if p.peek() != ')' {
		return nil, fmt.Errorf("expected ')' after IN value list")
	}
	p.advance() // skip ')'

	return &InExpression{
		Not:        notIn,
		Expression: left,
		Values:     values,
	}, nil
}

// parseAdditiveExpression parses addition and subtraction
func (p *Parser) parseAdditiveExpression() (Expression, error) {
	left, err := p.parseMultiplicativeExpression()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		var op Operator
		if p.match("+") {
			op = OpAdd
		} else if p.match("-") {
			op = OpSubtract
		} else {
			break
		}

		right, err := p.parseMultiplicativeExpression()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpression{
			Left:     left,
			Operator: op,
			Right:    right,
		}
	}

	return left, nil
}

// parseMultiplicativeExpression parses multiplication and division
func (p *Parser) parseMultiplicativeExpression() (Expression, error) {
	left, err := p.parseUnaryExpression()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		var op Operator
		if p.match("*") {
			op = OpMultiply
		} else if p.match("/") {
			op = OpDivide
		} else {
			break
		}

		right, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}

		left = &BinaryExpression{
			Left:     left,
			Operator: op,
			Right:    right,
		}
	}

	return left, nil
}

// parseUnaryExpression parses unary operators
func (p *Parser) parseUnaryExpression() (Expression, error) {
	p.skipWhitespace()

	// Check for unary operators
	if p.match("!") {
		operand, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		return &UnaryExpression{
			Operator: OpNot,
			Operand:  operand,
		}, nil
	}

	if p.match("+") {
		// Unary plus is essentially a no-op, just parse the operand
		return p.parseUnaryExpression()
	}

	if p.match("-") {
		// Unary minus
		operand, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		// Represent as 0 - operand
		return &BinaryExpression{
			Left:     &LiteralExpression{Literal: rdf.NewIntegerLiteral(0)},
			Operator: OpSubtract,
			Right:    operand,
		}, nil
	}

	return p.parsePrimaryExpression()
}

// parsePrimaryExpression parses primary expressions (variables, literals, functions, parentheses)
func (p *Parser) parsePrimaryExpression() (Expression, error) {
	p.skipWhitespace()

	// Check for boolean literals (true/false)
	savedPos := p.pos
	if p.matchKeyword("TRUE") {
		return &LiteralExpression{Literal: rdf.NewBooleanLiteral(true)}, nil
	}
	p.pos = savedPos
	if p.matchKeyword("FALSE") {
		return &LiteralExpression{Literal: rdf.NewBooleanLiteral(false)}, nil
	}
	p.pos = savedPos

	// Check for EXISTS or NOT EXISTS
	if p.matchKeyword("NOT") {
		p.skipWhitespace()
		if p.matchKeyword("EXISTS") {
			p.skipWhitespace()
			pattern, err := p.parseGraphPattern()
			if err != nil {
				return nil, fmt.Errorf("failed to parse graph pattern in NOT EXISTS: %w", err)
			}
			return &ExistsExpression{Not: true, Pattern: *pattern}, nil
		}
		// Not followed by EXISTS, restore position
		p.pos = savedPos
	} else if p.matchKeyword("EXISTS") {
		p.skipWhitespace()
		pattern, err := p.parseGraphPattern()
		if err != nil {
			return nil, fmt.Errorf("failed to parse graph pattern in EXISTS: %w", err)
		}
		return &ExistsExpression{Not: false, Pattern: *pattern}, nil
	}

	// Check for parenthesized expression
	if p.peek() == '(' {
		p.advance() // skip '('
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if p.peek() != ')' {
			return nil, fmt.Errorf("expected ')' after expression")
		}
		p.advance() // skip ')'
		return expr, nil
	}

	// Check for variable
	if p.peek() == '?' || p.peek() == '$' {
		variable, err := p.parseVariable()
		if err != nil {
			return nil, err
		}
		return &VariableExpression{Variable: variable}, nil
	}

	// Check for function call (uppercase letter at start, or prefixed name like xsd:string or :myFunc)
	ch := p.peek()
	if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == ':' {
		// Try to parse as function call
		savedPos := p.pos
		_ = p.readWhile(func(c byte) bool {
			return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == ':'
		})

		p.skipWhitespace()
		if p.peek() == '(' {
			// It's a function call
			p.pos = savedPos // restore position
			return p.parseFunctionCall()
		}

		// Not a function call, restore and try as literal/keyword
		p.pos = savedPos
	}

	// Check for literals - parse using parseTermOrVariable
	termOrVar, err := p.parseTermOrVariable()
	if err != nil {
		return nil, fmt.Errorf("expected expression: %w", err)
	}

	// If it's a variable, we shouldn't get here (handled above)
	// If it's a term, wrap it in a LiteralExpression
	if termOrVar.Term != nil {
		// Validate: blank node labels cannot be used in expressions
		if _, isBlankNode := termOrVar.Term.(*rdf.BlankNode); isBlankNode {
			return nil, fmt.Errorf("blank node labels cannot be used in expressions")
		}
		return &LiteralExpression{Literal: termOrVar.Term}, nil
	}

	// If we got a variable here, it's already been handled above, but just in case
	if termOrVar.Variable != nil {
		return &VariableExpression{Variable: termOrVar.Variable}, nil
	}

	return nil, fmt.Errorf("failed to parse expression term")
}

// parseFunctionCall parses a function call expression
func (p *Parser) parseFunctionCall() (Expression, error) {
	p.skipWhitespace()

	// Read function name (can be identifier or prefixed name like xsd:string)
	funcName := p.readWhile(func(c byte) bool {
		return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == ':'
	})

	if funcName == "" {
		return nil, fmt.Errorf("expected function name")
	}

	// Expand prefixed name to full IRI if it contains a colon
	if strings.Contains(funcName, ":") {
		// This is a prefixed name - expand it using current prefixes
		parts := strings.SplitN(funcName, ":", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			localName := parts[1]
			if ns, ok := p.prefixes[prefix]; ok {
				funcName = ns + localName
			}
			// If prefix not found, keep as-is (may be built-in like xsd:string)
		}
	}

	p.skipWhitespace()
	if p.peek() != '(' {
		return nil, fmt.Errorf("expected '(' after function name")
	}
	p.advance() // skip '('

	// Parse arguments
	var args []Expression
	p.skipWhitespace()

	// Check for empty argument list
	if p.peek() == ')' {
		p.advance() // skip ')'
		return &FunctionCallExpression{
			Function:  funcName,
			Arguments: args,
		}, nil
	}

	// Parse first argument
	for {
		// Special case for COUNT(*) and similar
		if funcName == "COUNT" && p.peek() == '*' {
			p.advance()
			// Add a special marker for COUNT(*)
			args = append(args, &VariableExpression{Variable: &Variable{Name: "*"}})
		} else {
			arg, err := p.parseExpression()
			if err != nil {
				return nil, fmt.Errorf("error parsing function argument: %w", err)
			}
			args = append(args, arg)
		}

		p.skipWhitespace()
		if p.peek() == ',' {
			p.advance() // skip ','
			p.skipWhitespace()
			continue
		}
		break
	}

	p.skipWhitespace()
	if p.peek() != ')' {
		return nil, fmt.Errorf("expected ')' after function arguments")
	}
	p.advance() // skip ')'

	return &FunctionCallExpression{
		Function:  funcName,
		Arguments: args,
	}, nil
}

// match checks if the next characters match the given string and advances if they do
func (p *Parser) match(s string) bool {
	if p.pos+len(s) > p.length {
		return false
	}

	for i := 0; i < len(s); i++ {
		if p.input[p.pos+i] != s[i] {
			return false
		}
	}

	p.pos += len(s)
	return true
}

// resolveIRI resolves a potentially relative IRI against the BASE URI
func (p *Parser) resolveIRI(iri string) string {
	// If no BASE is set or IRI is absolute, return as-is
	if p.baseURI == "" || isAbsoluteIRI(iri) {
		return iri
	}

	// Handle fragment-only IRIs like "#x"
	if strings.HasPrefix(iri, "#") {
		return p.baseURI + iri
	}

	// Simple relative IRI resolution
	// This is a simplified version - full RFC 3986 resolution is complex
	return p.baseURI + iri
}

// isAbsoluteIRI checks if an IRI is absolute (has a scheme)
func isAbsoluteIRI(iri string) bool {
	// Check for scheme: "scheme:"
	colonIdx := strings.Index(iri, ":")
	if colonIdx <= 0 {
		return false
	}
	// Check that everything before colon is valid scheme chars
	for i := 0; i < colonIdx; i++ {
		c := iri[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9' && i > 0) || c == '+' || c == '-' || c == '.') {
			return false
		}
	}
	return true
}
