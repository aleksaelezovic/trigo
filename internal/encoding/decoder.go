package encoding

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// TermDecoder handles decoding of RDF terms
type TermDecoder struct{}

// NewTermDecoder creates a new term decoder
func NewTermDecoder() *TermDecoder {
	return &TermDecoder{}
}

// DecodeTerm decodes an encoded term back to an rdf.Term
// For terms that require string lookup, stringValue should be provided
func (d *TermDecoder) DecodeTerm(encoded EncodedTerm, stringValue *string) (rdf.Term, error) {
	termType := GetTermType(encoded)

	switch termType {
	case rdf.TermTypeNamedNode:
		if stringValue == nil {
			return nil, fmt.Errorf("string value required for named node")
		}
		return rdf.NewNamedNode(*stringValue), nil

	case rdf.TermTypeBlankNode:
		if stringValue != nil {
			return rdf.NewBlankNode(*stringValue), nil
		}
		// Try to decode as numeric ID
		numericID := binary.BigEndian.Uint64(encoded[1:9])
		return rdf.NewBlankNode(strconv.FormatUint(numericID, 10)), nil

	case rdf.TermTypeStringLiteral:
		if stringValue != nil {
			return rdf.NewLiteral(*stringValue), nil
		}
		// Try to extract inline string
		// Find null terminator or end of data
		endIdx := 1
		for endIdx < EncodedTermSize && encoded[endIdx] != 0 {
			endIdx++
		}
		inlineStr := string(encoded[1:endIdx])
		return rdf.NewLiteral(inlineStr), nil

	case rdf.TermTypeLangStringLiteral:
		if stringValue == nil {
			return nil, fmt.Errorf("string value required for language-tagged literal")
		}
		// Split value@language
		for i := len(*stringValue) - 1; i >= 0; i-- {
			if (*stringValue)[i] == '@' {
				value := (*stringValue)[:i]
				lang := (*stringValue)[i+1:]
				return rdf.NewLiteralWithLanguage(value, lang), nil
			}
		}
		return rdf.NewLiteral(*stringValue), nil

	case rdf.TermTypeIntegerLiteral:
		value := int64(binary.BigEndian.Uint64(encoded[1:9])) // #nosec G115 - intentional bit-pattern conversion for binary decoding
		return rdf.NewIntegerLiteral(value), nil

	case rdf.TermTypeDecimalLiteral:
		bits := binary.BigEndian.Uint64(encoded[1:9])
		value := math.Float64frombits(bits)
		return rdf.NewLiteralWithDatatype(fmt.Sprintf("%g", value), rdf.XSDDecimal), nil

	case rdf.TermTypeDoubleLiteral:
		bits := binary.BigEndian.Uint64(encoded[1:9])
		value := math.Float64frombits(bits)
		return rdf.NewDoubleLiteral(value), nil

	case rdf.TermTypeBooleanLiteral:
		value := encoded[1] != 0
		return rdf.NewBooleanLiteral(value), nil

	case rdf.TermTypeDateTimeLiteral:
		nanos := int64(binary.BigEndian.Uint64(encoded[1:9])) // #nosec G115 - intentional bit-pattern conversion for timestamp decoding
		t := time.Unix(0, nanos)
		return rdf.NewDateTimeLiteral(t), nil

	case rdf.TermTypeDateLiteral:
		days := int64(binary.BigEndian.Uint64(encoded[1:9])) // #nosec G115 - intentional bit-pattern conversion for date decoding
		t := time.Unix(days*86400, 0)
		return rdf.NewLiteralWithDatatype(t.Format("2006-01-02"), rdf.XSDDate), nil

	case rdf.TermTypeDefaultGraph:
		return rdf.NewDefaultGraph(), nil

	default:
		return nil, fmt.Errorf("unknown term type: %d", termType)
	}
}
