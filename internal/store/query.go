package store

import (
	"fmt"

	"github.com/aleksaelezovic/trigo/internal/encoding"
	"github.com/aleksaelezovic/trigo/internal/storage"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// Pattern represents a triple or quad pattern with optional variables
type Pattern struct {
	Subject   interface{} // rdf.Term or Variable
	Predicate interface{} // rdf.Term or Variable
	Object    interface{} // rdf.Term or Variable
	Graph     interface{} // rdf.Term or Variable (nil means any graph)
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
	values map[string]encoding.EncodedTerm // internal encoded values
}

// NewBinding creates a new empty binding
func NewBinding() *Binding {
	return &Binding{
		Vars:   make(map[string]rdf.Term),
		values: make(map[string]encoding.EncodedTerm),
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
		txn.Rollback()
		return nil, err
	}

	// Create iterator
	it, err := txn.Scan(table, prefix, nil)
	if err != nil {
		txn.Rollback()
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
func (s *TripleStore) selectIndex(pattern *Pattern) (storage.Table, []int) {
	sBound := !isVariable(pattern.Subject)
	pBound := !isVariable(pattern.Predicate)
	oBound := !isVariable(pattern.Object)
	gBound := pattern.Graph != nil && !isVariable(pattern.Graph)

	// If graph is not specified or is a variable, prefer default graph indexes
	if !gBound {
		// Default graph indexes (SPO, POS, OSP)
		if sBound && pBound {
			return storage.TableSPO, []int{0, 1, 2} // S, P, O
		}
		if pBound && oBound {
			return storage.TablePOS, []int{0, 1, 2} // P, O, S
		}
		if oBound && sBound {
			return storage.TableOSP, []int{0, 1, 2} // O, S, P
		}
		if sBound {
			return storage.TableSPO, []int{0, 1, 2} // S, P, O
		}
		if pBound {
			return storage.TablePOS, []int{0, 1, 2} // P, O, S
		}
		if oBound {
			return storage.TableOSP, []int{0, 1, 2} // O, S, P
		}
		// No variables bound, use SPO
		return storage.TableSPO, []int{0, 1, 2}
	}

	// Named graph indexes (SPOG, POSG, OSPG, GSPO, GPOS, GOSP)
	if gBound && sBound && pBound {
		return storage.TableGSPO, []int{0, 1, 2, 3} // G, S, P, O
	}
	if gBound && pBound && oBound {
		return storage.TableGPOS, []int{0, 1, 2, 3} // G, P, O, S
	}
	if gBound && oBound && sBound {
		return storage.TableGOSP, []int{0, 1, 2, 3} // G, O, S, P
	}
	if gBound && sBound {
		return storage.TableGSPO, []int{0, 1, 2, 3} // G, S, P, O
	}
	if gBound && pBound {
		return storage.TableGPOS, []int{0, 1, 2, 3} // G, P, O, S
	}
	if gBound && oBound {
		return storage.TableGOSP, []int{0, 1, 2, 3} // G, O, S, P
	}
	if gBound {
		return storage.TableGSPO, []int{0, 1, 2, 3} // G, S, P, O
	}

	// Fallback to SPOG for mixed queries
	return storage.TableSPOG, []int{0, 1, 2, 3}
}

// buildScanPrefix builds a key prefix for scanning based on bound positions
func (s *TripleStore) buildScanPrefix(pattern *Pattern, keyPattern []int) ([]byte, error) {
	// Map pattern positions: 0=S, 1=P, 2=O, 3=G
	positions := make([]interface{}, 4)
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
func isVariable(v interface{}) bool {
	_, ok := v.(*Variable)
	return ok
}

// quadIterator implements QuadIterator
type quadIterator struct {
	store      *TripleStore
	txn        storage.Transaction
	it         storage.Iterator
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
	if len(key) < len(qi.keyPattern)*encoding.EncodedTermSize {
		return nil, fmt.Errorf("invalid key length: %d", len(key))
	}

	// Extract encoded terms
	terms := make([]encoding.EncodedTerm, len(qi.keyPattern))
	for i := 0; i < len(qi.keyPattern); i++ {
		offset := i * encoding.EncodedTermSize
		copy(terms[i][:], key[offset:offset+encoding.EncodedTermSize])
	}

	// Map back to S, P, O, G positions
	positions := make([]encoding.EncodedTerm, 4)
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
	qi.it.Close()
	return qi.txn.Rollback()
}

// decodeTerm decodes an encoded term back to an rdf.Term
func (s *TripleStore) decodeTerm(txn storage.Transaction, encoded encoding.EncodedTerm) (rdf.Term, error) {
	termType := encoding.GetTermType(encoded)
	decoder := encoding.NewTermDecoder()

	// For terms that need string lookup
	var stringValue *string
	if termType == rdf.TermTypeNamedNode || termType == rdf.TermTypeBlankNode ||
		termType == rdf.TermTypeStringLiteral || termType == rdf.TermTypeLangStringLiteral {

		str, err := txn.Get(storage.TableID2Str, encoded[1:])
		if err == nil {
			strVal := string(str)
			stringValue = &strVal
		}
	}

	return decoder.DecodeTerm(encoded, stringValue)
}
