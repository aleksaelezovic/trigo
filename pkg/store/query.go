package store

import (
	"fmt"

	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// Pattern represents a triple or quad pattern with optional variables
type Pattern struct {
	Subject   any // rdf.Term or Variable
	Predicate any // rdf.Term or Variable
	Object    any // rdf.Term or Variable
	Graph     any // rdf.Term or Variable (nil means any graph)
}

// Variable represents a SPARQL variable
type Variable struct {
	Name string
}

// NewVariable creates a new variable
func NewVariable(name string) *Variable {
	return &Variable{Name: name}
}

func (v *Variable) String() string {
	return "?" + v.Name
}

// Binding represents a variable binding
type Binding struct {
	Vars   map[string]rdf.Term
	values map[string]EncodedTerm // internal encoded values
}

// NewBinding creates a new empty binding
func NewBinding() *Binding {
	return &Binding{
		Vars:   make(map[string]rdf.Term),
		values: make(map[string]EncodedTerm),
	}
}

// Clone creates a copy of the binding
func (b *Binding) Clone() *Binding {
	newBinding := NewBinding()
	for k, v := range b.Vars {
		newBinding.Vars[k] = v
	}
	for k, v := range b.values {
		newBinding.values[k] = v
	}
	return newBinding
}

// QuadIterator iterates over quads matching a pattern
type QuadIterator interface {
	Next() bool
	Quad() (*rdf.Quad, error)
	Close() error
}

// BindingIterator iterates over variable bindings
type BindingIterator interface {
	Next() bool
	Binding() *Binding
	Close() error
}

// Query executes a pattern match and returns matching quads
func (s *TripleStore) Query(pattern *Pattern) (QuadIterator, error) {
	txn, err := s.storage.Begin(false)
	if err != nil {
		return nil, err
	}

	// Select the best index based on bound positions
	table, keyPattern := s.selectIndex(pattern)

	// Build the prefix for scanning
	prefix, err := s.buildScanPrefix(pattern, keyPattern)
	if err != nil {
		_ = txn.Rollback() // #nosec G104 - rollback error less important than original error
		return nil, err
	}

	// Create iterator
	it, err := txn.Scan(table, prefix, nil)
	if err != nil {
		_ = txn.Rollback() // #nosec G104 - rollback error less important than original error
		return nil, err
	}

	return &quadIterator{
		store:      s,
		txn:        txn,
		it:         it,
		pattern:    pattern,
		keyPattern: keyPattern,
	}, nil
}

// selectIndex chooses the best index based on which positions are bound
func (s *TripleStore) selectIndex(pattern *Pattern) (Table, []int) {
	sBound := !isVariable(pattern.Subject)
	pBound := !isVariable(pattern.Predicate)
	oBound := !isVariable(pattern.Object)
	gBound := pattern.Graph != nil && !isVariable(pattern.Graph)

	// If graph is not specified or is a variable, prefer default graph indexes
	if !gBound {
		// Default graph indexes (SPO, POS, OSP)
		// KeyPattern maps: key_position -> SPOG_position (S=0, P=1, O=2, G=3)
		if sBound && pBound {
			return TableSPO, []int{0, 1, 2} // Key order: S, P, O
		}
		if pBound && oBound {
			return TablePOS, []int{1, 2, 0} // Key order: P, O, S
		}
		if oBound && sBound {
			return TableOSP, []int{2, 0, 1} // Key order: O, S, P
		}
		if sBound {
			return TableSPO, []int{0, 1, 2} // Key order: S, P, O
		}
		if pBound {
			return TablePOS, []int{1, 2, 0} // Key order: P, O, S
		}
		if oBound {
			return TableOSP, []int{2, 0, 1} // Key order: O, S, P
		}
		// No variables bound, use SPO
		return TableSPO, []int{0, 1, 2}
	}

	// Named graph indexes (SPOG, POSG, OSPG, GSPO, GPOS, GOSP)
	// KeyPattern maps: key_position -> SPOG_position (S=0, P=1, O=2, G=3)
	if gBound && sBound && pBound {
		return TableGSPO, []int{3, 0, 1, 2} // Key order: G, S, P, O
	}
	if gBound && pBound && oBound {
		return TableGPOS, []int{3, 1, 2, 0} // Key order: G, P, O, S
	}
	if gBound && oBound && sBound {
		return TableGOSP, []int{3, 2, 0, 1} // Key order: G, O, S, P
	}
	if gBound && sBound {
		return TableGSPO, []int{3, 0, 1, 2} // Key order: G, S, P, O
	}
	if gBound && pBound {
		return TableGPOS, []int{3, 1, 2, 0} // Key order: G, P, O, S
	}
	if gBound && oBound {
		return TableGOSP, []int{3, 2, 0, 1} // Key order: G, O, S, P
	}
	if gBound {
		return TableGSPO, []int{3, 0, 1, 2} // Key order: G, S, P, O
	}

	// Fallback to SPOG for mixed queries
	return TableSPOG, []int{0, 1, 2, 3}
}

// buildScanPrefix builds a key prefix for scanning based on bound positions
func (s *TripleStore) buildScanPrefix(pattern *Pattern, keyPattern []int) ([]byte, error) {
	// Map pattern positions: 0=S, 1=P, 2=O, 3=G
	positions := make([]any, 4)
	positions[0] = pattern.Subject
	positions[1] = pattern.Predicate
	positions[2] = pattern.Object
	if pattern.Graph != nil {
		positions[3] = pattern.Graph
	} else {
		positions[3] = rdf.NewDefaultGraph()
	}

	// Build prefix from bound terms in key order
	var prefix []byte
	for _, idx := range keyPattern {
		if idx >= len(positions) {
			break
		}

		term := positions[idx]
		if isVariable(term) {
			// Stop at first variable
			break
		}

		// Encode the term
		encoded, _, err := s.encoder.EncodeTerm(term.(rdf.Term))
		if err != nil {
			return nil, err
		}

		prefix = append(prefix, encoded[:]...)
	}

	return prefix, nil
}

// isVariable checks if a value is a variable
func isVariable(v any) bool {
	_, ok := v.(*Variable)
	return ok
}

// quadIterator implements QuadIterator
type quadIterator struct {
	store      *TripleStore
	txn        Transaction
	it         Iterator
	pattern    *Pattern
	keyPattern []int
	closed     bool
}

func (qi *quadIterator) Next() bool {
	if qi.closed {
		return false
	}
	return qi.it.Next()
}

func (qi *quadIterator) Quad() (*rdf.Quad, error) {
	if qi.closed {
		return nil, fmt.Errorf("iterator closed")
	}

	key := qi.it.Key()
	if key == nil {
		return nil, fmt.Errorf("no current key")
	}

	// Decode key based on key pattern
	// Each encoded term is 17 bytes
	const encodedTermSize = 17
	if len(key) < len(qi.keyPattern)*encodedTermSize {
		return nil, fmt.Errorf("invalid key length: %d", len(key))
	}

	// Extract encoded terms
	terms := make([]EncodedTerm, len(qi.keyPattern))
	for i := 0; i < len(qi.keyPattern); i++ {
		offset := i * encodedTermSize
		copy(terms[i][:], key[offset:offset+encodedTermSize])
	}

	// Map back to S, P, O, G positions
	positions := make([]EncodedTerm, 4)
	for i, idx := range qi.keyPattern {
		positions[idx] = terms[i]
	}

	// Decode terms
	subject, err := qi.store.decodeTerm(qi.txn, positions[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode subject: %w", err)
	}

	predicate, err := qi.store.decodeTerm(qi.txn, positions[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode predicate: %w", err)
	}

	object, err := qi.store.decodeTerm(qi.txn, positions[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode object: %w", err)
	}

	var graph rdf.Term
	if len(qi.keyPattern) > 3 {
		graph, err = qi.store.decodeTerm(qi.txn, positions[3])
		if err != nil {
			return nil, fmt.Errorf("failed to decode graph: %w", err)
		}
	} else {
		graph = rdf.NewDefaultGraph()
	}

	return &rdf.Quad{
		Subject:   subject,
		Predicate: predicate,
		Object:    object,
		Graph:     graph,
	}, nil
}

func (qi *quadIterator) Close() error {
	if qi.closed {
		return nil
	}
	qi.closed = true
	_ = qi.it.Close() // #nosec G104 - iterator close error less critical than transaction rollback error
	return qi.txn.Rollback()
}

// decodeTerm decodes an encoded term back to an rdf.Term
func (s *TripleStore) decodeTerm(txn Transaction, encoded EncodedTerm) (rdf.Term, error) {
	termType := rdf.TermType(encoded[0])

	// For terms that need string lookup
	var stringValue *string
	if termType == rdf.TermTypeNamedNode || termType == rdf.TermTypeBlankNode ||
		termType == rdf.TermTypeStringLiteral || termType == rdf.TermTypeLangStringLiteral {

		str, err := txn.Get(TableID2Str, encoded[1:])
		if err == nil {
			strVal := string(str)
			stringValue = &strVal
		}
	}

	return s.decoder.DecodeTerm(encoded, stringValue)
}
