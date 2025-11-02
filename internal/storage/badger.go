package storage

import (
	"bytes"
	"fmt"

	"github.com/aleksaelezovic/trigo/pkg/store"
	badger "github.com/dgraph-io/badger/v4"
)

// BadgerStorage implements Storage using BadgerDB
type BadgerStorage struct {
	db *badger.DB
}

// NewBadgerStorage creates a new BadgerDB-backed storage
func NewBadgerStorage(path string) (*BadgerStorage, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Disable default logger

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	return &BadgerStorage{db: db}, nil
}

// Begin starts a new transaction
func (s *BadgerStorage) Begin(writable bool) (store.Transaction, error) {
	txn := s.db.NewTransaction(writable)
	return &BadgerTransaction{
		txn:      txn,
		writable: writable,
	}, nil
}

// Close closes the storage
func (s *BadgerStorage) Close() error {
	return s.db.Close()
}

// Sync flushes writes to disk
func (s *BadgerStorage) Sync() error {
	return s.db.Sync()
}

// BadgerTransaction implements Transaction using BadgerDB
type BadgerTransaction struct {
	txn      *badger.Txn
	writable bool
}

// Get retrieves a value by key
func (t *BadgerTransaction) Get(table store.Table, key []byte) ([]byte, error) {
	prefixedKey := store.PrefixKey(table, key)
	item, err := t.txn.Get(prefixedKey)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, store.ErrNotFound
		}
		return nil, err
	}

	var value []byte
	err = item.Value(func(val []byte) error {
		value = append([]byte{}, val...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Set stores a key-value pair
func (t *BadgerTransaction) Set(table store.Table, key, value []byte) error {
	if !t.writable {
		return store.ErrTransactionRO
	}

	prefixedKey := store.PrefixKey(table, key)
	return t.txn.Set(prefixedKey, value)
}

// Delete removes a key
func (t *BadgerTransaction) Delete(table store.Table, key []byte) error {
	if !t.writable {
		return store.ErrTransactionRO
	}

	prefixedKey := store.PrefixKey(table, key)
	return t.txn.Delete(prefixedKey)
}

// Scan iterates over a key range [start, end)
func (t *BadgerTransaction) Scan(table store.Table, start, end []byte) (store.Iterator, error) {
	opts := badger.DefaultIteratorOptions

	// Seek to start position
	var seekKey []byte
	var scanPrefix []byte
	tablePrefix := store.TablePrefix(table)

	if start != nil {
		seekKey = store.PrefixKey(table, start)
		// Use the start key as prefix to narrow down the scan
		scanPrefix = seekKey
	} else {
		seekKey = tablePrefix
		// Use the table prefix for full table scans
		scanPrefix = tablePrefix
	}

	opts.Prefix = scanPrefix
	it := t.txn.NewIterator(opts)

	// Calculate end key with prefix
	var endKey []byte
	if end != nil {
		endKey = store.PrefixKey(table, end)
	}

	return &BadgerIterator{
		it:         it,
		prefix:     tablePrefix, // Use table prefix for stripping
		scanPrefix: scanPrefix,  // Use full prefix for validation
		endKey:     endKey,
		seekKey:    seekKey,
		started:    false,
		hasValue:   false,
	}, nil
}

// Commit commits the transaction
func (t *BadgerTransaction) Commit() error {
	return t.txn.Commit()
}

// Rollback rolls back the transaction
func (t *BadgerTransaction) Rollback() error {
	t.txn.Discard()
	return nil
}

// BadgerIterator implements Iterator using BadgerDB
type BadgerIterator struct {
	it         *badger.Iterator
	prefix     []byte // Table prefix for stripping from keys
	scanPrefix []byte // Full prefix used for BadgerDB filtering
	endKey     []byte
	seekKey    []byte
	started    bool
	hasValue   bool
}

// Next advances to the next item
func (i *BadgerIterator) Next() bool {
	if !i.started {
		i.it.Seek(i.seekKey)
		i.started = true
	} else {
		i.it.Next()
	}

	// Check if iterator is still valid
	if !i.it.Valid() {
		i.hasValue = false
		return false
	}

	// Check if we've reached the end key
	if i.endKey != nil {
		if bytes.Compare(i.it.Item().Key(), i.endKey) >= 0 {
			i.hasValue = false
			return false
		}
	}

	i.hasValue = true
	return true
}

// Key returns the current key (without the table prefix)
func (i *BadgerIterator) Key() []byte {
	if !i.hasValue {
		return nil
	}

	key := i.it.Item().Key()
	// Remove table prefix
	if len(key) > len(i.prefix) {
		return key[len(i.prefix):]
	}
	return nil
}

// Value returns the current value
func (i *BadgerIterator) Value() ([]byte, error) {
	if !i.hasValue {
		return nil, store.ErrNotFound
	}

	var value []byte
	err := i.it.Item().Value(func(val []byte) error {
		value = append([]byte{}, val...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return value, nil
}

// Close closes the iterator
func (i *BadgerIterator) Close() error {
	i.it.Close()
	return nil
}
