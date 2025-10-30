package encoding

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
	"github.com/zeebo/xxh3"
)

const (
	// Maximum size for inline strings (16 bytes of UTF-8)
	MaxInlineStringSize = 16

	// Encoded term size (type byte + 16 bytes for 128-bit hash or inline data)
	EncodedTermSize = 17
)

// EncodedTerm represents a term encoded as a type byte followed by up to 16 bytes of data
type EncodedTerm [EncodedTermSize]byte

// TermEncoder handles encoding and decoding of RDF terms
type TermEncoder struct {
	// Hash function for strings (xxhash3 128-bit)
}

func NewTermEncoder() *TermEncoder {
	return &TermEncoder{}
}

// Hash128 computes a 128-bit xxhash3 hash of the input string
func (e *TermEncoder) Hash128(s string) [16]byte {
	hash := xxh3.Hash128([]byte(s))
	var result [16]byte
	// xxh3.Hash128 returns a uint128-like type, we need to extract the bytes
	binary.BigEndian.PutUint64(result[0:8], hash.Hi)
	binary.BigEndian.PutUint64(result[8:16], hash.Lo)
	return result
}

// EncodeTerm encodes an RDF term into a fixed-size byte array
// Returns the encoded term and optionally a string to store in id2str table
func (e *TermEncoder) EncodeTerm(term rdf.Term) (EncodedTerm, *string, error) {
	var encoded EncodedTerm

	switch t := term.(type) {
	case *rdf.NamedNode:
		return e.encodeNamedNode(t)
	case *rdf.BlankNode:
		return e.encodeBlankNode(t)
	case *rdf.Literal:
		return e.encodeLiteral(t)
	case *rdf.DefaultGraph:
		return e.encodeDefaultGraph()
	default:
		return encoded, nil, fmt.Errorf("unknown term type: %T", term)
	}
}

func (e *TermEncoder) encodeNamedNode(node *rdf.NamedNode) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeNamedNode)

	// Always hash IRIs (using 128-bit xxhash3)
	hash := e.Hash128(node.IRI)
	copy(encoded[1:], hash[:])

	return encoded, &node.IRI, nil
}

func (e *TermEncoder) encodeBlankNode(node *rdf.BlankNode) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeBlankNode)

	// Try to parse as numeric ID
	if num, err := strconv.ParseUint(node.ID, 10, 64); err == nil {
		// Store as inline numeric ID (big endian)
		binary.BigEndian.PutUint64(encoded[1:9], num)
		// Zero out remaining bytes
		for i := 9; i < EncodedTermSize; i++ {
			encoded[i] = 0
		}
		return encoded, nil, nil
	}

	// Hash non-numeric blank node IDs
	hash := e.Hash128(node.ID)
	copy(encoded[1:], hash[:])

	return encoded, &node.ID, nil
}

func (e *TermEncoder) encodeLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	// Check for typed literals with special encoding
	if lit.Datatype != nil {
		switch lit.Datatype.IRI {
		case rdf.XSDInteger.IRI:
			return e.encodeIntegerLiteral(lit)
		case rdf.XSDDecimal.IRI:
			return e.encodeDecimalLiteral(lit)
		case rdf.XSDDouble.IRI:
			return e.encodeDoubleLiteral(lit)
		case rdf.XSDBoolean.IRI:
			return e.encodeBooleanLiteral(lit)
		case rdf.XSDDateTime.IRI:
			return e.encodeDateTimeLiteral(lit)
		case rdf.XSDDate.IRI:
			return e.encodeDateLiteral(lit)
		}
	}

	// Language-tagged string
	if lit.Language != "" {
		return e.encodeLangStringLiteral(lit)
	}

	// Plain string literal
	return e.encodeStringLiteral(lit)
}

func (e *TermEncoder) encodeStringLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeStringLiteral)

	if len(lit.Value) <= MaxInlineStringSize {
		// Inline small strings
		copy(encoded[1:], []byte(lit.Value))
		// Zero out remaining bytes
		for i := 1 + len(lit.Value); i < EncodedTermSize; i++ {
			encoded[i] = 0
		}
		return encoded, nil, nil
	}

	// Hash large strings
	hash := e.Hash128(lit.Value)
	copy(encoded[1:], hash[:])

	return encoded, &lit.Value, nil
}

func (e *TermEncoder) encodeLangStringLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeLangStringLiteral)

	// Combine value and language tag for hashing
	combined := lit.Value + "@" + lit.Language
	hash := e.Hash128(combined)
	copy(encoded[1:], hash[:])

	return encoded, &combined, nil
}

func (e *TermEncoder) encodeIntegerLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeIntegerLiteral)

	value, err := strconv.ParseInt(lit.Value, 10, 64)
	if err != nil {
		return encoded, nil, fmt.Errorf("invalid integer literal: %w", err)
	}

	// Store as big endian signed integer
	binary.BigEndian.PutUint64(encoded[1:9], uint64(value)) // #nosec G115 - intentional bit-pattern conversion for binary encoding
	// Zero out remaining bytes
	for i := 9; i < EncodedTermSize; i++ {
		encoded[i] = 0
	}

	return encoded, nil, nil
}

func (e *TermEncoder) encodeDecimalLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeDecimalLiteral)

	// Parse as float and store
	value, err := strconv.ParseFloat(lit.Value, 64)
	if err != nil {
		return encoded, nil, fmt.Errorf("invalid decimal literal: %w", err)
	}

	binary.BigEndian.PutUint64(encoded[1:9], math.Float64bits(value))
	// Zero out remaining bytes
	for i := 9; i < EncodedTermSize; i++ {
		encoded[i] = 0
	}

	return encoded, nil, nil
}

func (e *TermEncoder) encodeDoubleLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeDoubleLiteral)

	value, err := strconv.ParseFloat(lit.Value, 64)
	if err != nil {
		return encoded, nil, fmt.Errorf("invalid double literal: %w", err)
	}

	binary.BigEndian.PutUint64(encoded[1:9], math.Float64bits(value))
	// Zero out remaining bytes
	for i := 9; i < EncodedTermSize; i++ {
		encoded[i] = 0
	}

	return encoded, nil, nil
}

func (e *TermEncoder) encodeBooleanLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeBooleanLiteral)

	value, err := strconv.ParseBool(lit.Value)
	if err != nil {
		return encoded, nil, fmt.Errorf("invalid boolean literal: %w", err)
	}

	if value {
		encoded[1] = 1
	} else {
		encoded[1] = 0
	}

	// Zero out remaining bytes
	for i := 2; i < EncodedTermSize; i++ {
		encoded[i] = 0
	}

	return encoded, nil, nil
}

func (e *TermEncoder) encodeDateTimeLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeDateTimeLiteral)

	// Parse RFC3339 datetime
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(lit.Value))
	if err != nil {
		return encoded, nil, fmt.Errorf("invalid datetime literal: %w", err)
	}

	// Store as Unix timestamp (nanoseconds since epoch)
	nanos := t.UnixNano()
	binary.BigEndian.PutUint64(encoded[1:9], uint64(nanos)) // #nosec G115 - intentional bit-pattern conversion for timestamp encoding

	// Zero out remaining bytes
	for i := 9; i < EncodedTermSize; i++ {
		encoded[i] = 0
	}

	return encoded, nil, nil
}

func (e *TermEncoder) encodeDateLiteral(lit *rdf.Literal) (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeDateLiteral)

	// Parse date (assuming YYYY-MM-DD format)
	t, err := time.Parse("2006-01-02", strings.TrimSpace(lit.Value))
	if err != nil {
		return encoded, nil, fmt.Errorf("invalid date literal: %w", err)
	}

	// Store as Unix timestamp (days since epoch)
	days := t.Unix() / 86400
	binary.BigEndian.PutUint64(encoded[1:9], uint64(days)) // #nosec G115 - intentional bit-pattern conversion for date encoding

	// Zero out remaining bytes
	for i := 9; i < EncodedTermSize; i++ {
		encoded[i] = 0
	}

	return encoded, nil, nil
}

func (e *TermEncoder) encodeDefaultGraph() (EncodedTerm, *string, error) {
	var encoded EncodedTerm
	encoded[0] = byte(rdf.TermTypeDefaultGraph)

	// Zero out remaining bytes
	for i := 1; i < EncodedTermSize; i++ {
		encoded[i] = 0
	}

	return encoded, nil, nil
}

// EncodeQuadKey encodes a quad key for one of the 11 indexes
// Returns a big-endian byte array for lexicographic sorting
func (e *TermEncoder) EncodeQuadKey(terms ...EncodedTerm) []byte {
	result := make([]byte, 0, len(terms)*EncodedTermSize)
	for _, term := range terms {
		result = append(result, term[:]...)
	}
	return result
}

// GetTermType extracts the type from an encoded term
func GetTermType(encoded EncodedTerm) rdf.TermType {
	return rdf.TermType(encoded[0])
}
