package store

import (
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// EncodedTerm represents a term encoded as a type byte followed by up to 16 bytes of data
// This is defined here to be used by both the encoder and decoder interfaces
type EncodedTerm [17]byte

// TermEncoder handles encoding of RDF terms into a compact binary format
type TermEncoder interface {
	// EncodeTerm encodes an RDF term into a fixed-size byte array
	// Returns the encoded term and optionally a string to store in id2str table
	EncodeTerm(term rdf.Term) (EncodedTerm, *string, error)

	// EncodeQuadKey encodes a quad key for one of the indexes
	// Returns a big-endian byte array for lexicographic sorting
	EncodeQuadKey(terms ...EncodedTerm) []byte
}

// TermDecoder handles decoding of RDF terms from binary format
type TermDecoder interface {
	// DecodeTerm decodes an encoded term back to an rdf.Term
	// For terms that require string lookup, stringValue should be provided
	DecodeTerm(encoded EncodedTerm, stringValue *string) (rdf.Term, error)
}
