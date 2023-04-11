package ezdb

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rs/zerolog"
	lmdb "wellquite.org/golmdb"
)

const mode = os.FileMode(0644)

type Option func(option *options) error

type options struct {
	numReaders *uint
	numDbs     *uint
	batchSize  *uint
	log        *zerolog.Logger
}

func WithNumReaders(numReaders uint) Option {
	return func(option *options) error {
		option.numReaders = &numReaders
		return nil
	}
}

func WithNumDBs(numDbs uint) Option {
	return func(option *options) error {
		option.numDbs = &numDbs
		return nil
	}
}

func WithBatchSize(batchSize uint) Option {
	return func(option *options) error {
		option.batchSize = &batchSize
		return nil
	}
}

func WithLogger(logger zerolog.Logger) Option {
	return func(option *options) error {
		option.log = &logger
		return nil
	}
}

type Client struct {
	path    string
	options *options

	db       *lmdb.LMDBClient
	initOnce sync.Once
}

func New(path string, opts ...Option) (*Client, error) {
	o := &options{}
	for _, opt := range opts {
		err := opt(o)
		if err != nil {
			return nil, fmt.Errorf("failed to set options: %w", err)
		}
	}

	// Default values
	if o.numReaders == nil {
		o.numReaders = new(uint)
		*o.numReaders = 8
	}
	if o.numDbs == nil {
		o.numDbs = new(uint)
		*o.numDbs = 1
	}
	if o.batchSize == nil {
		o.batchSize = new(uint)
		*o.batchSize = 1
	}
	if o.log == nil {
		o.log = new(zerolog.Logger)
		*o.log = zerolog.Nop()
	}

	return &Client{
		path:    path,
		options: o,
	}, nil
}

func (db *Client) init() error {
	// Check if directory exists, if not create it.
	if _, err := os.Stat(db.path); os.IsNotExist(err) {
		err = os.MkdirAll(db.path, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	// Open DB.
	newDB, err := lmdb.NewLMDB(*db.options.log, db.path, mode, *db.options.numReaders, *db.options.numDbs, lmdb.EnvironmentFlag(0), *db.options.batchSize)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}

	db.db = newDB
	return nil
}

func (db *Client) Close() {
	db.db.TerminateSync()
}

type DBRef[K, V any] struct {
	id      string
	ownerDB *Client
	// TODO: reuse the gob encoder here.
	// Also, since typeinfo is hardcoded here, maybe better to replace gob with raw bytes.
	// Worth looking into go-bolt for their pure byte implementation.
}

func (ref *DBRef[K, V]) Init(refID string, db *Client) error {
	var err error
	db.initOnce.Do(func() {
		err = db.init()
	})
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	err = db.db.Update(func(txn *lmdb.ReadWriteTxn) error {
		_, err := txn.DBRef(refID, lmdb.DatabaseFlag(0x40000))
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to open db ref: %w", err)
	}

	*ref = DBRef[K, V]{
		id:      refID,
		ownerDB: db,
	}

	return nil
}

func (ref *DBRef[K, V]) Put(key *K, val *V) (err error) {
	err = ref.ownerDB.db.Update(func(txn *lmdb.ReadWriteTxn) error {
		dbRef, err := txn.DBRef(ref.id, lmdb.DatabaseFlag(0))
		if err != nil {
			return fmt.Errorf("failed to get db ref: %w", err)
		}

		// Encode the key.
		keyBuf, err := encode(key)
		if err != nil {
			return fmt.Errorf("failed to encode key: %w", err)
		}

		// Encode the value.
		valBuf, err := encode(val)
		if err != nil {
			return fmt.Errorf("failed to encode value: %w", err)
		}

		err = txn.Put(dbRef, keyBuf.Bytes(), valBuf.Bytes(), lmdb.PutFlag(0))
		if err != nil {
			return fmt.Errorf("failed to put key/value pair: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (ref *DBRef[K, V]) Get(key *K) (val *V, err error) {
	err = ref.ownerDB.db.View(func(txn *lmdb.ReadOnlyTxn) error {
		dbRef, err := txn.DBRef(ref.id, lmdb.DatabaseFlag(0))
		if err != nil {
			return fmt.Errorf("failed to get db ref: %w", err)
		}

		// Encode the key.
		keyBuf, err := encode(key)
		if err != nil {
			return fmt.Errorf("failed to encode key: %w", err)
		}

		// Get the value.
		valBytes, err := txn.Get(dbRef, keyBuf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to get key: %w", err)
		}

		// Decode the value.
		err = decode(&val, bytes.NewReader(valBytes))
		if err != nil {
			return fmt.Errorf("failed to decode value: %w", err)
		}

		return nil
	})
	if err != nil {
		return new(V), err
	}

	return val, nil
}

func encode[T any](val *T) (buf bytes.Buffer, err error) {
	encoder := gob.NewEncoder(&buf)
	err = encoder.Encode(val)
	if err != nil {
		return buf, err
	}

	return buf, nil
}

// Decodes a value from a reader into a pointer to a value.
// Will try and fail if the decoded type is not assignable to the thing we're decoding into.
func decode[T any](val *T, r io.Reader) error {
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(val)
	if err != nil {
		return err
	}

	return nil
}
