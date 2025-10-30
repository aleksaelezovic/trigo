package rdf

import (
	"encoding/binary"
	"fmt"
	"math"
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
	Value    string
	Language string     // for language-tagged strings
	Datatype *NamedNode // for typed literals
}

func NewLiteral(value string) *Literal {
	return &Literal{Value: value}
}

func NewLiteralWithLanguage(value, language string) *Literal {
	return &Literal{Value: value, Language: language}
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

func NewIntegerLiteral(value int64) *Literal {
	return NewLiteralWithDatatype(fmt.Sprintf("%d", value), XSDInteger)
}

func NewDoubleLiteral(value float64) *Literal {
	return NewLiteralWithDatatype(fmt.Sprintf("%g", value), XSDDouble)
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
	binary.BigEndian.PutUint64(buf, uint64(value))
	return buf
}

func DecodeInt64BigEndian(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

func EncodeFloat64BigEndian(value float64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, math.Float64bits(value))
	return buf
}

func DecodeFloat64BigEndian(buf []byte) float64 {
	return math.Float64frombits(binary.BigEndian.Uint64(buf))
}
