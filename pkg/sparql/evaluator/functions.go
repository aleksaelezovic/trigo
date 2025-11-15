package evaluator

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/aleksaelezovic/trigo/pkg/sparql/parser"
	"github.com/aleksaelezovic/trigo/pkg/store"
)

// evaluateFunctionCall evaluates a function call expression
func (e *Evaluator) evaluateFunctionCall(expr *parser.FunctionCallExpression, binding *store.Binding) (rdf.Term, error) {
	funcName := strings.ToUpper(expr.Function)

	switch funcName {
	// Type checking functions
	case "BOUND":
		return e.evaluateBound(expr.Arguments, binding)
	case "ISIRI", "ISURI":
		return e.evaluateIsIRI(expr.Arguments, binding)
	case "ISBLANK":
		return e.evaluateIsBlank(expr.Arguments, binding)
	case "ISLITERAL":
		return e.evaluateIsLiteral(expr.Arguments, binding)
	case "ISNUMERIC":
		return e.evaluateIsNumeric(expr.Arguments, binding)

	// Value extraction functions
	case "STR":
		return e.evaluateStr(expr.Arguments, binding)
	case "LANG":
		return e.evaluateLang(expr.Arguments, binding)
	case "DATATYPE":
		return e.evaluateDatatype(expr.Arguments, binding)

	// String functions
	case "STRLEN":
		return e.evaluateStrLen(expr.Arguments, binding)
	case "SUBSTR":
		return e.evaluateSubStr(expr.Arguments, binding)
	case "UCASE":
		return e.evaluateUCase(expr.Arguments, binding)
	case "LCASE":
		return e.evaluateLCase(expr.Arguments, binding)
	case "CONCAT":
		return e.evaluateConcat(expr.Arguments, binding)
	case "CONTAINS":
		return e.evaluateContains(expr.Arguments, binding)
	case "STRSTARTS":
		return e.evaluateStrStarts(expr.Arguments, binding)
	case "STRENDS":
		return e.evaluateStrEnds(expr.Arguments, binding)
	case "REGEX":
		return e.evaluateRegex(expr.Arguments, binding)
	case "LANGMATCHES":
		return e.evaluateLangMatches(expr.Arguments, binding)
	case "SAMETERM":
		return e.evaluateSameTerm(expr.Arguments, binding)

	// Numeric functions
	case "ABS":
		return e.evaluateAbs(expr.Arguments, binding)
	case "CEIL":
		return e.evaluateCeil(expr.Arguments, binding)
	case "FLOOR":
		return e.evaluateFloor(expr.Arguments, binding)
	case "ROUND":
		return e.evaluateRound(expr.Arguments, binding)

	default:
		// Check if it's a type casting function (IRI-based)
		if strings.HasPrefix(funcName, "HTTP://WWW.W3.ORG/2001/XMLSCHEMA#") {
			datatype := funcName // Full IRI
			return e.evaluateTypeCast(expr.Arguments, binding, datatype)
		}
		return nil, fmt.Errorf("unsupported function: %s", funcName)
	}
}

// Type checking functions

func (e *Evaluator) evaluateBound(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("BOUND requires exactly 1 argument")
	}

	// BOUND is special - it doesn't evaluate the argument, just checks if the variable is bound
	varExpr, ok := args[0].(*parser.VariableExpression)
	if !ok {
		return nil, fmt.Errorf("BOUND requires a variable argument")
	}

	_, exists := binding.Vars[varExpr.Variable.Name]
	return rdf.NewBooleanLiteral(exists), nil
}

func (e *Evaluator) evaluateIsIRI(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isIRI requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isIRI := term.(*rdf.NamedNode)
	return rdf.NewBooleanLiteral(isIRI), nil
}

func (e *Evaluator) evaluateIsBlank(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isBlank requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isBlank := term.(*rdf.BlankNode)
	return rdf.NewBooleanLiteral(isBlank), nil
}

func (e *Evaluator) evaluateIsLiteral(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isLiteral requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isLiteral := term.(*rdf.Literal)
	return rdf.NewBooleanLiteral(isLiteral), nil
}

func (e *Evaluator) evaluateIsNumeric(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("isNumeric requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	_, isNumeric := e.extractNumeric(term)
	return rdf.NewBooleanLiteral(isNumeric), nil
}

// Value extraction functions

func (e *Evaluator) evaluateStr(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("STR requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	switch t := term.(type) {
	case *rdf.NamedNode:
		return rdf.NewLiteral(t.IRI), nil
	case *rdf.Literal:
		return rdf.NewLiteral(t.Value), nil
	case *rdf.BlankNode:
		return nil, fmt.Errorf("STR cannot be applied to blank nodes")
	default:
		return nil, fmt.Errorf("STR: unsupported term type")
	}
}

func (e *Evaluator) evaluateLang(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("LANG requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	lit, ok := term.(*rdf.Literal)
	if !ok {
		return rdf.NewLiteral(""), nil
	}

	return rdf.NewLiteral(lit.Language), nil
}

func (e *Evaluator) evaluateDatatype(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("DATATYPE requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	lit, ok := term.(*rdf.Literal)
	if !ok {
		return nil, fmt.Errorf("DATATYPE can only be applied to literals")
	}

	if lit.Datatype != nil {
		return lit.Datatype, nil
	}

	// Default to xsd:string if no datatype
	return rdf.NewNamedNode("http://www.w3.org/2001/XMLSchema#string"), nil
}

// String functions

func (e *Evaluator) evaluateStrLen(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("STRLEN requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	return rdf.NewIntegerLiteral(int64(len(str))), nil
}

func (e *Evaluator) evaluateSubStr(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("SUBSTR requires 2 or 3 arguments")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	startTerm, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	start, ok := e.extractNumeric(startTerm)
	if !ok {
		return nil, fmt.Errorf("SUBSTR start position must be numeric")
	}

	// SPARQL uses 1-based indexing
	startIdx := int(start) - 1
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(str) {
		return rdf.NewLiteral(""), nil
	}

	if len(args) == 3 {
		lengthTerm, err := e.Evaluate(args[2], binding)
		if err != nil {
			return nil, err
		}

		length, ok := e.extractNumeric(lengthTerm)
		if !ok {
			return nil, fmt.Errorf("SUBSTR length must be numeric")
		}

		endIdx := startIdx + int(length)
		if endIdx > len(str) {
			endIdx = len(str)
		}

		return rdf.NewLiteral(str[startIdx:endIdx]), nil
	}

	return rdf.NewLiteral(str[startIdx:]), nil
}

func (e *Evaluator) evaluateUCase(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("UCASE requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	return rdf.NewLiteral(strings.ToUpper(str)), nil
}

func (e *Evaluator) evaluateLCase(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("LCASE requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	str, err := e.extractString(term)
	if err != nil {
		return nil, err
	}

	return rdf.NewLiteral(strings.ToLower(str)), nil
}

func (e *Evaluator) evaluateConcat(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) == 0 {
		return rdf.NewLiteral(""), nil
	}

	var result strings.Builder
	for _, arg := range args {
		term, err := e.Evaluate(arg, binding)
		if err != nil {
			return nil, err
		}

		str, err := e.extractString(term)
		if err != nil {
			return nil, err
		}

		result.WriteString(str)
	}

	return rdf.NewLiteral(result.String()), nil
}

func (e *Evaluator) evaluateContains(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("CONTAINS requires exactly 2 arguments")
	}

	term1, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	term2, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	str1, err := e.extractString(term1)
	if err != nil {
		return nil, err
	}

	str2, err := e.extractString(term2)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(strings.Contains(str1, str2)), nil
}

func (e *Evaluator) evaluateStrStarts(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("STRSTARTS requires exactly 2 arguments")
	}

	term1, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	term2, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	str1, err := e.extractString(term1)
	if err != nil {
		return nil, err
	}

	str2, err := e.extractString(term2)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(strings.HasPrefix(str1, str2)), nil
}

func (e *Evaluator) evaluateStrEnds(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("STRENDS requires exactly 2 arguments")
	}

	term1, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	term2, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	str1, err := e.extractString(term1)
	if err != nil {
		return nil, err
	}

	str2, err := e.extractString(term2)
	if err != nil {
		return nil, err
	}

	return rdf.NewBooleanLiteral(strings.HasSuffix(str1, str2)), nil
}

// Numeric functions

func (e *Evaluator) evaluateAbs(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("ABS requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("ABS requires numeric argument")
	}

	return e.createNumericLiteral(math.Abs(val), term, term), nil
}

func (e *Evaluator) evaluateCeil(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("CEIL requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("CEIL requires numeric argument")
	}

	return rdf.NewIntegerLiteral(int64(math.Ceil(val))), nil
}

func (e *Evaluator) evaluateFloor(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("FLOOR requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("FLOOR requires numeric argument")
	}

	return rdf.NewIntegerLiteral(int64(math.Floor(val))), nil
}

func (e *Evaluator) evaluateRound(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("ROUND requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	val, ok := e.extractNumeric(term)
	if !ok {
		return nil, fmt.Errorf("ROUND requires numeric argument")
	}

	return rdf.NewIntegerLiteral(int64(math.Round(val))), nil
}

func (e *Evaluator) evaluateRegex(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	// REGEX(text, pattern) or REGEX(text, pattern, flags)
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("REGEX requires 2 or 3 arguments")
	}

	// Evaluate text argument
	textTerm, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}
	text, err := e.extractString(textTerm)
	if err != nil {
		return nil, fmt.Errorf("REGEX text argument: %w", err)
	}

	// Evaluate pattern argument
	patternTerm, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}
	pattern, err := e.extractString(patternTerm)
	if err != nil {
		return nil, fmt.Errorf("REGEX pattern argument: %w", err)
	}

	// Evaluate flags argument (optional)
	var flags string
	if len(args) == 3 {
		flagsTerm, err := e.Evaluate(args[2], binding)
		if err != nil {
			return nil, err
		}
		flags, err = e.extractString(flagsTerm)
		if err != nil {
			return nil, fmt.Errorf("REGEX flags argument: %w", err)
		}
	}

	// Process flags
	// SPARQL flags: i (case-insensitive), m (multiline), s (dotall), x (extended/ignore whitespace), q (quote/literal)
	// Go regexp uses different syntax:
	//   - i: prepend (?i) to pattern
	//   - m: prepend (?m) to pattern (changes ^ and $ behavior)
	//   - s: prepend (?s) to pattern (. matches newlines)
	//   - x: prepend (?x) to pattern (ignore whitespace and allow comments)
	//   - q: escape all regex metacharacters using QuoteMeta
	var hasQuote bool
	var flagPrefix string
	if flags != "" {
		flagPrefix = "(?"
		for _, flag := range flags {
			switch flag {
			case 'i', 'm', 's', 'x':
				flagPrefix += string(flag)
			case 'q':
				hasQuote = true
			default:
				return nil, fmt.Errorf("unsupported REGEX flag: %c", flag)
			}
		}
		flagPrefix += ")"

		// Apply quote flag to escape metacharacters
		if hasQuote {
			pattern = regexp.QuoteMeta(pattern)
		}

		// Prepend flag modifiers if any
		if len(flagPrefix) > 2 {
			pattern = flagPrefix + pattern
		}
	}

	// Compile and match
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	matched := re.MatchString(text)
	return rdf.NewBooleanLiteral(matched), nil
}

func (e *Evaluator) evaluateLangMatches(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	// langMatches(language-tag, language-range)
	if len(args) != 2 {
		return nil, fmt.Errorf("langMatches requires exactly 2 arguments")
	}

	// Evaluate language tag argument
	tagTerm, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}
	tag, err := e.extractString(tagTerm)
	if err != nil {
		return nil, fmt.Errorf("langMatches tag argument: %w", err)
	}

	// Evaluate language range argument
	rangeTerm, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}
	langRange, err := e.extractString(rangeTerm)
	if err != nil {
		return nil, fmt.Errorf("langMatches range argument: %w", err)
	}

	// Simplified language matching per SPARQL spec:
	// - "*" matches any non-empty tag
	// - Exact match (case-insensitive)
	// - Prefix match: "de" matches "de-DE", "de-CH", etc.
	tag = strings.ToLower(tag)
	langRange = strings.ToLower(langRange)

	// "*" matches any non-empty language tag
	if langRange == "*" {
		return rdf.NewBooleanLiteral(tag != ""), nil
	}

	// Exact match
	if tag == langRange {
		return rdf.NewBooleanLiteral(true), nil
	}

	// Prefix match: range must be followed by "-"
	// e.g., "de" matches "de-DE" but not "deu"
	if strings.HasPrefix(tag, langRange+"-") {
		return rdf.NewBooleanLiteral(true), nil
	}

	return rdf.NewBooleanLiteral(false), nil
}

func (e *Evaluator) evaluateSameTerm(args []parser.Expression, binding *store.Binding) (rdf.Term, error) {
	// sameTerm(term1, term2) - strict equality (no type coercion)
	if len(args) != 2 {
		return nil, fmt.Errorf("sameTerm requires exactly 2 arguments")
	}

	term1, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	term2, err := e.Evaluate(args[1], binding)
	if err != nil {
		return nil, err
	}

	// sameTerm is true only if the terms are exactly the same
	// (same type, same value, same language tag, same datatype)
	result := e.termsEqual(term1, term2)
	return rdf.NewBooleanLiteral(result), nil
}

func (e *Evaluator) evaluateTypeCast(args []parser.Expression, binding *store.Binding, datatypeIRI string) (rdf.Term, error) {
	// Type casting: xsd:type(value)
	if len(args) != 1 {
		return nil, fmt.Errorf("type cast requires exactly 1 argument")
	}

	term, err := e.Evaluate(args[0], binding)
	if err != nil {
		return nil, err
	}

	// Extract the value to cast
	var value string
	switch t := term.(type) {
	case *rdf.Literal:
		value = t.Value
	case *rdf.NamedNode:
		value = t.IRI
	case *rdf.BlankNode:
		return nil, fmt.Errorf("cannot cast blank node to %s", datatypeIRI)
	default:
		return nil, fmt.Errorf("cannot cast term type %T to %s", term, datatypeIRI)
	}

	// Create a new literal with the target datatype
	// The value string is kept as-is (SPARQL spec allows this for type casting)
	return rdf.NewLiteralWithDatatype(value, rdf.NewNamedNode(datatypeIRI)), nil
}

// termsEqual checks strict RDF term equality
func (e *Evaluator) termsEqual(t1, t2 rdf.Term) bool {
	// Compare types first
	if t1.Type() != t2.Type() {
		return false
	}

	switch v1 := t1.(type) {
	case *rdf.NamedNode:
		v2 := t2.(*rdf.NamedNode)
		return v1.IRI == v2.IRI

	case *rdf.BlankNode:
		v2 := t2.(*rdf.BlankNode)
		return v1.ID == v2.ID

	case *rdf.Literal:
		v2 := t2.(*rdf.Literal)
		// Value must match
		if v1.Value != v2.Value {
			return false
		}
		// Language tag must match
		if v1.Language != v2.Language {
			return false
		}
		// Datatype must match
		if v1.Datatype == nil && v2.Datatype == nil {
			return true
		}
		if v1.Datatype == nil || v2.Datatype == nil {
			return false
		}
		return v1.Datatype.IRI == v2.Datatype.IRI

	default:
		return false
	}
}

// Helper function

func (e *Evaluator) extractString(term rdf.Term) (string, error) {
	switch t := term.(type) {
	case *rdf.Literal:
		return t.Value, nil
	case *rdf.NamedNode:
		return t.IRI, nil
	default:
		return "", fmt.Errorf("cannot extract string from term type: %T", term)
	}
}
