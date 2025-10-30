package parser

import (
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// Query represents a SPARQL query
type Query struct {
	QueryType QueryType
	Select    *SelectQuery
	Construct *ConstructQuery
	Ask       *AskQuery
	Describe  *DescribeQuery
}

// QueryType represents the type of SPARQL query
type QueryType int

const (
	QueryTypeSelect QueryType = iota
	QueryTypeConstruct
	QueryTypeAsk
	QueryTypeDescribe
)

// SelectQuery represents a SELECT query
type SelectQuery struct {
	Variables  []*Variable      // Variables to select (* for all)
	Distinct   bool             // DISTINCT modifier
	Where      *GraphPattern    // WHERE clause
	OrderBy    []*OrderCondition // ORDER BY clause
	Limit      *int             // LIMIT clause
	Offset     *int             // OFFSET clause
}

// ConstructQuery represents a CONSTRUCT query
type ConstructQuery struct {
	Template []*TriplePattern // CONSTRUCT template
	Where    *GraphPattern    // WHERE clause
}

// AskQuery represents an ASK query
type AskQuery struct {
	Where *GraphPattern // WHERE clause
}

// DescribeQuery represents a DESCRIBE query
type DescribeQuery struct {
	Resources []*rdf.NamedNode // Resources to describe
	Where     *GraphPattern     // WHERE clause (optional)
}

// GraphPattern represents a graph pattern
type GraphPattern struct {
	Type     GraphPatternType
	Patterns []*TriplePattern // For basic graph patterns
	Filters  []*Filter         // FILTER expressions
	Binds    []*Bind           // BIND expressions
	Children []*GraphPattern   // For complex patterns (UNION, OPTIONAL, etc.)
	Graph    *GraphTerm        // For GRAPH patterns
}

// GraphPatternType represents the type of graph pattern
type GraphPatternType int

const (
	GraphPatternTypeBasic GraphPatternType = iota
	GraphPatternTypeUnion
	GraphPatternTypeOptional
	GraphPatternTypeGraph
	GraphPatternTypeMinus
)

// TriplePattern represents a triple pattern with possible variables
type TriplePattern struct {
	Subject   TermOrVariable
	Predicate TermOrVariable
	Object    TermOrVariable
}

// TermOrVariable can be either an RDF term or a variable
type TermOrVariable struct {
	Term     rdf.Term
	Variable *Variable
}

// IsVariable returns true if this is a variable
func (t *TermOrVariable) IsVariable() bool {
	return t.Variable != nil
}

// Variable represents a SPARQL variable
type Variable struct {
	Name string
}

// GraphTerm represents a graph name (can be IRI or variable)
type GraphTerm struct {
	IRI      *rdf.NamedNode
	Variable *Variable
}

// Filter represents a FILTER expression
type Filter struct {
	Expression Expression
}

// Bind represents a BIND expression (assigns an expression to a variable)
type Bind struct {
	Expression Expression
	Variable   *Variable
}

// Expression represents a SPARQL expression
type Expression interface {
	expressionNode()
}

// BinaryExpression represents a binary operation
type BinaryExpression struct {
	Left     Expression
	Operator Operator
	Right    Expression
}

func (e *BinaryExpression) expressionNode() {}

// UnaryExpression represents a unary operation
type UnaryExpression struct {
	Operator Operator
	Operand  Expression
}

func (e *UnaryExpression) expressionNode() {}

// VariableExpression represents a variable in an expression
type VariableExpression struct {
	Variable *Variable
}

func (e *VariableExpression) expressionNode() {}

// LiteralExpression represents a literal value in an expression
type LiteralExpression struct {
	Literal rdf.Term
}

func (e *LiteralExpression) expressionNode() {}

// FunctionCallExpression represents a function call
type FunctionCallExpression struct {
	Function  string
	Arguments []Expression
}

func (e *FunctionCallExpression) expressionNode() {}

// Operator represents an operator in expressions
type Operator int

const (
	// Logical operators
	OpAnd Operator = iota
	OpOr
	OpNot

	// Comparison operators
	OpEqual
	OpNotEqual
	OpLessThan
	OpLessThanOrEqual
	OpGreaterThan
	OpGreaterThanOrEqual

	// Arithmetic operators
	OpAdd
	OpSubtract
	OpMultiply
	OpDivide

	// String operators
	OpRegex
	OpStr
	OpLang
	OpDatatype

	// Numeric operators
	OpIsNumeric
	OpAbs
	OpCeil
	OpFloor
	OpRound
)

// OrderCondition represents an ORDER BY condition
type OrderCondition struct {
	Expression Expression
	Ascending  bool
}
