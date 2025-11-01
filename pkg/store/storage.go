package store

import (
	"errors"
)

var (
	ErrNotFound      = errors.New("key not found")
	ErrTransactionRO = errors.New("transaction is read-only")
)

// Storage is the interface for the underlying key-value store
type Storage interface {
	// Begin starts a new transaction
	Begin(writable bool) (Transaction, error)

	// Close closes the storage
	Close() error

	// Sync flushes writes to disk
	Sync() error
}

// Transaction represents a database transaction with snapshot isolation
type Transaction interface {
	// Get retrieves a value by key
	Get(table Table, key []byte) ([]byte, error)

	// Set stores a key-value pair
	Set(table Table, key, value []byte) error

	// Delete removes a key
	Delete(table Table, key []byte) error

	// Scan iterates over a key range [start, end)
	// If start is nil, begins from the first key
	// If end is nil, scans until the last key
	Scan(table Table, start, end []byte) (Iterator, error)

	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error
}

// Iterator iterates over key-value pairs
type Iterator interface {
	// Next advances to the next item
	Next() bool

	// Key returns the current key
	Key() []byte

	// Value returns the current value
	Value() ([]byte, error)

	// Close closes the iterator
	Close() error
}

// Table represents a logical table/column family in the storage
type Table byte

const (
	// Metadata table: hash -> string
	TableID2Str Table = iota

	// Default graph indexes (3 permutations)
	TableSPO
	TablePOS
	TableOSP

	// Named graph indexes (6 permutations)
	TableSPOG
	TablePOSG
	TableOSPG
	TableGSPO
	TableGPOS
	TableGOSP

	// Named graphs metadata
	TableGraphs

	// Total number of tables
	TableCount
)

func (t Table) String() string {
	switch t {
	case TableID2Str:
		return "id2str"
	case TableSPO:
		return "spo"
	case TablePOS:
		return "pos"
	case TableOSP:
		return "osp"
	case TableSPOG:
		return "spog"
	case TablePOSG:
		return "posg"
	case TableOSPG:
		return "ospg"
	case TableGSPO:
		return "gspo"
	case TableGPOS:
		return "gpos"
	case TableGOSP:
		return "gosp"
	case TableGraphs:
		return "graphs"
	default:
		return "unknown"
	}
}

// TablePrefix returns a byte prefix for a table to namespace keys
func TablePrefix(table Table) []byte {
	return []byte{byte(table)}
}

// PrefixKey adds a table prefix to a key
func PrefixKey(table Table, key []byte) []byte {
	prefix := TablePrefix(table)
	result := make([]byte, len(prefix)+len(key))
	copy(result, prefix)
	copy(result[len(prefix):], key)
	return result
}
