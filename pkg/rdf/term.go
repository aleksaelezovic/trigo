package rdf

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

// TermType represents the type of an RDF term
type TermType byte

const (
	// Core RDF types
	TermTypeNamedNode TermType = iota + 1
	TermTypeBlankNode
	TermTypeLiteral
	TermTypeDefaultGraph
	TermTypeQuotedTriple // RDF 1.2: Triple terms

	// Literal subtypes
	TermTypeStringLiteral
	TermTypeLangStringLiteral
	TermTypeIntegerLiteral
	TermTypeDecimalLiteral
	TermTypeDoubleLiteral
	TermTypeBooleanLiteral
	TermTypeDateTimeLiteral
	TermTypeDateLiteral
	TermTypeTimeLiteral
	TermTypeDurationLiteral
)

// Term represents an RDF term (IRI, blank node, or literal)
type Term interface {
	Type() TermType
	String() string
	Equals(other Term) bool
}

// NamedNode represents an IRI
type NamedNode struct {
	IRI string
}

func NewNamedNode(iri string) *NamedNode {
	return &NamedNode{IRI: iri}
}

func (n *NamedNode) Type() TermType {
	return TermTypeNamedNode
}

func (n *NamedNode) String() string {
	return fmt.Sprintf("<%s>", n.IRI)
}

func (n *NamedNode) Equals(other Term) bool {
	if on, ok := other.(*NamedNode); ok {
		return n.IRI == on.IRI
	}
	return false
}

// BlankNode represents a blank node
type BlankNode struct {
	ID string
}

func NewBlankNode(id string) *BlankNode {
	return &BlankNode{ID: id}
}

func (b *BlankNode) Type() TermType {
	return TermTypeBlankNode
}

func (b *BlankNode) String() string {
	return fmt.Sprintf("_:%s", b.ID)
}

func (b *BlankNode) Equals(other Term) bool {
	if ob, ok := other.(*BlankNode); ok {
		return b.ID == ob.ID
	}
	return false
}

// Literal represents an RDF literal
type Literal struct {
	Value     string
	Language  string     // for language-tagged strings
	Direction string     // RDF 1.2: text direction ("ltr", "rtl", or "")
	Datatype  *NamedNode // for typed literals
}

func NewLiteral(value string) *Literal {
	return &Literal{Value: value}
}

func NewLiteralWithLanguage(value, language string) *Literal {
	return &Literal{Value: value, Language: language}
}

// NewLiteralWithLanguageAndDirection creates a literal with language and direction (RDF 1.2)
func NewLiteralWithLanguageAndDirection(value, language, direction string) *Literal {
	return &Literal{Value: value, Language: language, Direction: direction}
}

func NewLiteralWithDatatype(value string, datatype *NamedNode) *Literal {
	return &Literal{Value: value, Datatype: datatype}
}

func (l *Literal) Type() TermType {
	return TermTypeLiteral
}

func (l *Literal) String() string {
	result := fmt.Sprintf(`"%s"`, l.Value)
	if l.Language != "" {
		result += "@" + l.Language
		// RDF 1.2: Add direction if present
		if l.Direction != "" {
			result += "--" + l.Direction
		}
	} else if l.Datatype != nil {
		result += "^^" + l.Datatype.String()
	}
	return result
}

func (l *Literal) Equals(other Term) bool {
	if ol, ok := other.(*Literal); ok {
		if l.Value != ol.Value {
			return false
		}
		if l.Language != ol.Language {
			return false
		}
		// RDF 1.2: Compare direction
		if l.Direction != ol.Direction {
			return false
		}
		if l.Datatype == nil && ol.Datatype == nil {
			return true
		}
		if l.Datatype != nil && ol.Datatype != nil {
			return l.Datatype.Equals(ol.Datatype)
		}
		return false
	}
	return false
}

// DefaultGraph represents the default graph
type DefaultGraph struct{}

func NewDefaultGraph() *DefaultGraph {
	return &DefaultGraph{}
}

func (d *DefaultGraph) Type() TermType {
	return TermTypeDefaultGraph
}

func (d *DefaultGraph) String() string {
	return "DEFAULT"
}

func (d *DefaultGraph) Equals(other Term) bool {
	_, ok := other.(*DefaultGraph)
	return ok
}

// QuotedTriple represents an RDF 1.2 quoted triple (triple term)
// Can be used as subject or object in other triples
type QuotedTriple struct {
	Subject   Term
	Predicate Term
	Object    Term
}

// NewQuotedTriple creates a new quoted triple with validation
func NewQuotedTriple(subject, predicate, object Term) (*QuotedTriple, error) {
	// Validate subject: must be IRI, BlankNode, or QuotedTriple
	switch subject.(type) {
	case *NamedNode, *BlankNode, *QuotedTriple:
		// Valid
	default:
		return nil, fmt.Errorf("quoted triple subject must be IRI, blank node, or quoted triple, got %T", subject)
	}

	// Validate predicate: must be IRI (no QuotedTriple allowed)
	if _, ok := predicate.(*NamedNode); !ok {
		return nil, fmt.Errorf("quoted triple predicate must be IRI, got %T", predicate)
	}

	// Object can be any term (IRI, BlankNode, Literal, or QuotedTriple)

	return &QuotedTriple{
		Subject:   subject,
		Predicate: predicate,
		Object:    object,
	}, nil
}

func (q *QuotedTriple) Type() TermType {
	return TermTypeQuotedTriple
}

func (q *QuotedTriple) String() string {
	// Use compact notation without the outer angle brackets for the terms
	subj := q.Subject.String()
	pred := q.Predicate.String()
	obj := q.Object.String()
	return fmt.Sprintf("<< %s %s %s >>", subj, pred, obj)
}

func (q *QuotedTriple) Equals(other Term) bool {
	if oq, ok := other.(*QuotedTriple); ok {
		return q.Subject.Equals(oq.Subject) &&
			q.Predicate.Equals(oq.Predicate) &&
			q.Object.Equals(oq.Object)
	}
	return false
}

// TripleTerm represents an RDF 1.2 triple term <<( s p o )>> (N-Triples 1.2 syntax in Turtle)
// Triple terms are NOT automatically reified when used as subjects/objects
type TripleTerm struct {
	Subject   Term
	Predicate Term
	Object    Term
}

func (t *TripleTerm) Type() TermType {
	return TermTypeQuotedTriple // Triple terms are a form of quoted triples
}

func (t *TripleTerm) String() string {
	return fmt.Sprintf("<<( %s %s %s )>>",
		t.Subject.String(),
		t.Predicate.String(),
		t.Object.String())
}

func (t *TripleTerm) Equals(other Term) bool {
	if ot, ok := other.(*TripleTerm); ok {
		return t.Subject.Equals(ot.Subject) &&
			t.Predicate.Equals(ot.Predicate) &&
			t.Object.Equals(ot.Object)
	}
	return false
}

// ReifiedTriple represents a quoted triple with an explicit identifier (RDF 1.2 reification)
// Syntax: << s p o ~ identifier >>
// This is used internally during parsing to track that a quoted triple has an identifier
type ReifiedTriple struct {
	Identifier Term          // The identifier (IRI or blank node) for this reified triple
	Triple     *QuotedTriple // The underlying quoted triple
}

func (r *ReifiedTriple) Type() TermType {
	return TermTypeQuotedTriple // Reified triples are a form of quoted triples
}

func (r *ReifiedTriple) String() string {
	return fmt.Sprintf("<< %s %s %s ~ %s >>",
		r.Triple.Subject.String(),
		r.Triple.Predicate.String(),
		r.Triple.Object.String(),
		r.Identifier.String())
}

func (r *ReifiedTriple) Equals(other Term) bool {
	if or, ok := other.(*ReifiedTriple); ok {
		return r.Identifier.Equals(or.Identifier) && r.Triple.Equals(or.Triple)
	}
	return false
}

// Triple represents an RDF triple (subject, predicate, object)
type Triple struct {
	Subject   Term
	Predicate Term
	Object    Term
}

func NewTriple(subject, predicate, object Term) *Triple {
	return &Triple{
		Subject:   subject,
		Predicate: predicate,
		Object:    object,
	}
}

func (t *Triple) String() string {
	return fmt.Sprintf("%s %s %s .", t.Subject, t.Predicate, t.Object)
}

// Quad represents an RDF quad (subject, predicate, object, graph)
type Quad struct {
	Subject   Term
	Predicate Term
	Object    Term
	Graph     Term
}

func NewQuad(subject, predicate, object, graph Term) *Quad {
	return &Quad{
		Subject:   subject,
		Predicate: predicate,
		Object:    object,
		Graph:     graph,
	}
}

func (q *Quad) String() string {
	return fmt.Sprintf("%s %s %s %s .", q.Subject, q.Predicate, q.Object, q.Graph)
}

// Helper functions for common XSD datatypes
var (
	XSDString   = NewNamedNode("http://www.w3.org/2001/XMLSchema#string")
	XSDInteger  = NewNamedNode("http://www.w3.org/2001/XMLSchema#integer")
	XSDDecimal  = NewNamedNode("http://www.w3.org/2001/XMLSchema#decimal")
	XSDDouble   = NewNamedNode("http://www.w3.org/2001/XMLSchema#double")
	XSDBoolean  = NewNamedNode("http://www.w3.org/2001/XMLSchema#boolean")
	XSDDateTime = NewNamedNode("http://www.w3.org/2001/XMLSchema#dateTime")
	XSDDate     = NewNamedNode("http://www.w3.org/2001/XMLSchema#date")
	XSDTime     = NewNamedNode("http://www.w3.org/2001/XMLSchema#time")
	XSDDuration = NewNamedNode("http://www.w3.org/2001/XMLSchema#duration")
)

// RDF 1.2 vocabulary constants
var (
	RDFDirLangString = NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#dirLangString")
	RDFReifies       = NewNamedNode("http://www.w3.org/1999/02/22-rdf-syntax-ns#reifies")
)

func NewIntegerLiteral(value int64) *Literal {
	return NewLiteralWithDatatype(fmt.Sprintf("%d", value), XSDInteger)
}

func NewDoubleLiteral(value float64) *Literal {
	// Format doubles preserving decimal point and using scientific notation for large/small values
	var str string
	// If it's a whole number, ensure it has .0
	if value == float64(int64(value)) && value < 1e15 && value > -1e15 {
		str = fmt.Sprintf("%.1f", value)
	} else {
		// Use %g for scientific notation, but ensure decimal point exists
		str = fmt.Sprintf("%g", value)
		if !strings.Contains(str, ".") && !strings.Contains(str, "e") && !strings.Contains(str, "E") {
			str = str + ".0"
		}
	}
	return NewLiteralWithDatatype(str, XSDDouble)
}

func NewDecimalLiteral(value float64) *Literal {
	// Decimals should always have a decimal point, preserve trailing zeros
	str := fmt.Sprintf("%.1f", value)
	// If the value has more precision, show it
	if value != float64(int64(value*10)/10) {
		str = fmt.Sprintf("%f", value)
		// Trim excessive trailing zeros but keep at least one digit after decimal
		str = strings.TrimRight(str, "0")
		if strings.HasSuffix(str, ".") {
			str = str + "0"
		}
	}
	return NewLiteralWithDatatype(str, XSDDecimal)
}

func NewBooleanLiteral(value bool) *Literal {
	return NewLiteralWithDatatype(fmt.Sprintf("%t", value), XSDBoolean)
}

func NewDateTimeLiteral(value time.Time) *Literal {
	return NewLiteralWithDatatype(value.Format(time.RFC3339), XSDDateTime)
}

// Utility functions for encoding numeric values
func EncodeInt64BigEndian(value int64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(value)) // #nosec G115 - intentional bit-pattern conversion for binary encoding
	return buf
}

func DecodeInt64BigEndian(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf)) // #nosec G115 - intentional bit-pattern conversion for binary decoding
}

func EncodeFloat64BigEndian(value float64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, math.Float64bits(value))
	return buf
}

func DecodeFloat64BigEndian(buf []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(buf))
}
