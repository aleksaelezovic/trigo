package store

import (
	"bytes"
	"fmt"

	"github.com/aleksaelezovic/trigo/internal/encoding"
	"github.com/aleksaelezovic/trigo/internal/storage"
	"github.com/aleksaelezovic/trigo/pkg/rdf"
)

// TripleStore manages the RDF triplestore with 11 indexes
type TripleStore struct {
	storage storage.Storage
	encoder *encoding.TermEncoder
}

// NewTripleStore creates a new triplestore
func NewTripleStore(storage storage.Storage) *TripleStore {
	return &TripleStore{
		storage: storage,
		encoder: encoding.NewTermEncoder(),
	}
}

// Close closes the triplestore
func (s *TripleStore) Close() error {
	return s.storage.Close()
}

// InsertQuad inserts a quad into the store
func (s *TripleStore) InsertQuad(quad *rdf.Quad) error {
	txn, err := s.storage.Begin(true)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	if err := s.insertQuadInTxn(txn, quad); err != nil {
		return err
	}

	return txn.Commit()
}

// InsertTriple inserts a triple into the default graph
func (s *TripleStore) InsertTriple(triple *rdf.Triple) error {
	quad := &rdf.Quad{
		Subject:   triple.Subject,
		Predicate: triple.Predicate,
		Object:    triple.Object,
		Graph:     rdf.NewDefaultGraph(),
	}
	return s.InsertQuad(quad)
}

// insertQuadInTxn inserts a quad within an existing transaction
func (s *TripleStore) insertQuadInTxn(txn storage.Transaction, quad *rdf.Quad) error {
	// Encode terms
	subjEnc, subjStr, err := s.encoder.EncodeTerm(quad.Subject)
	if err != nil {
		return fmt.Errorf("failed to encode subject: %w", err)
	}

	predEnc, predStr, err := s.encoder.EncodeTerm(quad.Predicate)
	if err != nil {
		return fmt.Errorf("failed to encode predicate: %w", err)
	}

	objEnc, objStr, err := s.encoder.EncodeTerm(quad.Object)
	if err != nil {
		return fmt.Errorf("failed to encode object: %w", err)
	}

	graphEnc, graphStr, err := s.encoder.EncodeTerm(quad.Graph)
	if err != nil {
		return fmt.Errorf("failed to encode graph: %w", err)
	}

	// Store strings in id2str table
	if err := s.storeString(txn, subjEnc, subjStr); err != nil {
		return err
	}
	if err := s.storeString(txn, predEnc, predStr); err != nil {
		return err
	}
	if err := s.storeString(txn, objEnc, objStr); err != nil {
		return err
	}
	if err := s.storeString(txn, graphEnc, graphStr); err != nil {
		return err
	}

	// Empty value for all index entries
	emptyValue := []byte{}

	// Check if this is the default graph
	isDefaultGraph := quad.Graph.Type() == rdf.TermTypeDefaultGraph

	if isDefaultGraph {
		// Insert into default graph indexes (3 permutations)
		if err := txn.Set(storage.TableSPO, s.encoder.EncodeQuadKey(subjEnc, predEnc, objEnc), emptyValue); err != nil {
			return err
		}
		if err := txn.Set(storage.TablePOS, s.encoder.EncodeQuadKey(predEnc, objEnc, subjEnc), emptyValue); err != nil {
			return err
		}
		if err := txn.Set(storage.TableOSP, s.encoder.EncodeQuadKey(objEnc, subjEnc, predEnc), emptyValue); err != nil {
			return err
		}
	}

	// Insert into named graph indexes (6 permutations)
	// These are used for both named graphs and can serve as backup for default graph queries
	if err := txn.Set(storage.TableSPOG, s.encoder.EncodeQuadKey(subjEnc, predEnc, objEnc, graphEnc), emptyValue); err != nil {
		return err
	}
	if err := txn.Set(storage.TablePOSG, s.encoder.EncodeQuadKey(predEnc, objEnc, subjEnc, graphEnc), emptyValue); err != nil {
		return err
	}
	if err := txn.Set(storage.TableOSPG, s.encoder.EncodeQuadKey(objEnc, subjEnc, predEnc, graphEnc), emptyValue); err != nil {
		return err
	}
	if err := txn.Set(storage.TableGSPO, s.encoder.EncodeQuadKey(graphEnc, subjEnc, predEnc, objEnc), emptyValue); err != nil {
		return err
	}
	if err := txn.Set(storage.TableGPOS, s.encoder.EncodeQuadKey(graphEnc, predEnc, objEnc, subjEnc), emptyValue); err != nil {
		return err
	}
	if err := txn.Set(storage.TableGOSP, s.encoder.EncodeQuadKey(graphEnc, objEnc, subjEnc, predEnc), emptyValue); err != nil {
		return err
	}

	// Track named graph
	if !isDefaultGraph {
		if err := txn.Set(storage.TableGraphs, graphEnc[:], emptyValue); err != nil {
			return err
		}
	}

	return nil
}

// storeString stores a string in the id2str table if provided
func (s *TripleStore) storeString(txn storage.Transaction, encoded encoding.EncodedTerm, str *string) error {
	if str == nil {
		return nil
	}

	// Use the encoded term (which contains the hash) as the key
	key := encoded[1:] // Skip the type byte, use the hash/data portion
	value := []byte(*str)

	// Check if already exists to avoid unnecessary writes
	existing, err := txn.Get(storage.TableID2Str, key)
	if err == nil && bytes.Equal(existing, value) {
		return nil
	}
	if err != nil && err != storage.ErrNotFound {
		return err
	}

	return txn.Set(storage.TableID2Str, key, value)
}

// DeleteQuad deletes a quad from the store
func (s *TripleStore) DeleteQuad(quad *rdf.Quad) error {
	txn, err := s.storage.Begin(true)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	if err := s.deleteQuadInTxn(txn, quad); err != nil {
		return err
	}

	return txn.Commit()
}

// DeleteTriple deletes a triple from the default graph
func (s *TripleStore) DeleteTriple(triple *rdf.Triple) error {
	quad := &rdf.Quad{
		Subject:   triple.Subject,
		Predicate: triple.Predicate,
		Object:    triple.Object,
		Graph:     rdf.NewDefaultGraph(),
	}
	return s.DeleteQuad(quad)
}

// deleteQuadInTxn deletes a quad within an existing transaction
func (s *TripleStore) deleteQuadInTxn(txn storage.Transaction, quad *rdf.Quad) error {
	// Encode terms
	subjEnc, _, err := s.encoder.EncodeTerm(quad.Subject)
	if err != nil {
		return fmt.Errorf("failed to encode subject: %w", err)
	}

	predEnc, _, err := s.encoder.EncodeTerm(quad.Predicate)
	if err != nil {
		return fmt.Errorf("failed to encode predicate: %w", err)
	}

	objEnc, _, err := s.encoder.EncodeTerm(quad.Object)
	if err != nil {
		return fmt.Errorf("failed to encode object: %w", err)
	}

	graphEnc, _, err := s.encoder.EncodeTerm(quad.Graph)
	if err != nil {
		return fmt.Errorf("failed to encode graph: %w", err)
	}

	// Check if this is the default graph
	isDefaultGraph := quad.Graph.Type() == rdf.TermTypeDefaultGraph

	if isDefaultGraph {
		// Delete from default graph indexes
		if err := txn.Delete(storage.TableSPO, s.encoder.EncodeQuadKey(subjEnc, predEnc, objEnc)); err != nil {
			return err
		}
		if err := txn.Delete(storage.TablePOS, s.encoder.EncodeQuadKey(predEnc, objEnc, subjEnc)); err != nil {
			return err
		}
		if err := txn.Delete(storage.TableOSP, s.encoder.EncodeQuadKey(objEnc, subjEnc, predEnc)); err != nil {
			return err
		}
	}

	// Delete from named graph indexes
	if err := txn.Delete(storage.TableSPOG, s.encoder.EncodeQuadKey(subjEnc, predEnc, objEnc, graphEnc)); err != nil {
		return err
	}
	if err := txn.Delete(storage.TablePOSG, s.encoder.EncodeQuadKey(predEnc, objEnc, subjEnc, graphEnc)); err != nil {
		return err
	}
	if err := txn.Delete(storage.TableOSPG, s.encoder.EncodeQuadKey(objEnc, subjEnc, predEnc, graphEnc)); err != nil {
		return err
	}
	if err := txn.Delete(storage.TableGSPO, s.encoder.EncodeQuadKey(graphEnc, subjEnc, predEnc, objEnc)); err != nil {
		return err
	}
	if err := txn.Delete(storage.TableGPOS, s.encoder.EncodeQuadKey(graphEnc, predEnc, objEnc, subjEnc)); err != nil {
		return err
	}
	if err := txn.Delete(storage.TableGOSP, s.encoder.EncodeQuadKey(graphEnc, objEnc, subjEnc, predEnc)); err != nil {
		return err
	}

	// Note: We don't remove from graphs table or id2str table
	// as they may be referenced by other quads (no garbage collection)

	return nil
}

// ContainsQuad checks if a quad exists in the store
func (s *TripleStore) ContainsQuad(quad *rdf.Quad) (bool, error) {
	txn, err := s.storage.Begin(false)
	if err != nil {
		return false, err
	}
	defer txn.Rollback()

	// Encode terms
	subjEnc, _, err := s.encoder.EncodeTerm(quad.Subject)
	if err != nil {
		return false, err
	}

	predEnc, _, err := s.encoder.EncodeTerm(quad.Predicate)
	if err != nil {
		return false, err
	}

	objEnc, _, err := s.encoder.EncodeTerm(quad.Object)
	if err != nil {
		return false, err
	}

	graphEnc, _, err := s.encoder.EncodeTerm(quad.Graph)
	if err != nil {
		return false, err
	}

	// Check in SPOG index
	key := s.encoder.EncodeQuadKey(subjEnc, predEnc, objEnc, graphEnc)
	_, err = txn.Get(storage.TableSPOG, key)
	if err == storage.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// Count returns the approximate number of quads in the store
func (s *TripleStore) Count() (int64, error) {
	txn, err := s.storage.Begin(false)
	if err != nil {
		return 0, err
	}
	defer txn.Rollback()

	// Count entries in SPOG index (primary index for quads)
	it, err := txn.Scan(storage.TableSPOG, nil, nil)
	if err != nil {
		return 0, err
	}
	defer it.Close()

	count := int64(0)
	for it.Next() {
		count++
	}

	return count, nil
}
