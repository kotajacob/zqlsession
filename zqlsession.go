// License: LGPL-3.0-only
// (c) 2024 Dakota Walsh <kota@nilsu.org>
package zqlsession

import (
	"context"
	"log"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// SQLitexStore represents the session store.
type SQLitexStore struct {
	db          *sqlitex.Pool
	stopCleanup chan bool
}

// New returns a new SQLitexStore instance, with a background cleanup goroutine
// that runs every 5 minutes to remove expired session data.
func New(db *sqlitex.Pool) *SQLitexStore {
	return NewWithCleanupInterval(db, 5*time.Minute)
}

// NewWithCleanupInterval returns a new SQLitexStore instance. The cleanupInterval
// parameter controls how frequently expired session data is removed by the
// background cleanup goroutine. Setting it to 0 prevents the cleanup goroutine
// from running (i.e. expired sessions will not be removed).
func NewWithCleanupInterval(db *sqlitex.Pool, cleanupInterval time.Duration) *SQLitexStore {
	p := &SQLitexStore{db: db}
	if cleanupInterval > 0 {
		go p.startCleanup(cleanupInterval)
	}
	return p
}

// Find returns the data for a given session token from the SQLitexStore instance.
// If the session token is not found or is expired, the returned exists flag will
// be set to false.
func (p *SQLitexStore) Find(token string) ([]byte, bool, error) {
	conn, err := p.db.Take(context.Background())
	if err != nil {
		return nil, false, err
	}
	defer p.db.Put(conn)

	var found bool
	var b []byte
	err = sqlitex.Execute(conn,
		"SELECT data FROM sessions WHERE token = $1 AND julianday('now') < expiry",
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				found = true
				b = make([]byte, stmt.ColumnLen(0))
				stmt.ColumnBytes(0, b)
				return nil
			},
			Args: []any{token},
		})

	if !found {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Commit adds a session token and data to the SQLitexStore instance with the
// given expiry time. If the session token already exists, then the data and expiry
// time are updated.
func (p *SQLitexStore) Commit(token string, b []byte, expiry time.Time) error {
	conn, err := p.db.Take(context.Background())
	if err != nil {
		return err
	}
	defer p.db.Put(conn)

	err = sqlitex.Execute(conn,
		"REPLACE INTO sessions (token, data, expiry) VALUES ($1, $2, julianday($3))",
		&sqlitex.ExecOptions{
			Args: []any{token, b, expiry.UTC().Format("2006-01-02T15:04:05.999")},
		})
	return err
}

// Delete removes a session token and corresponding data from the SQLitexStore
// instance.
func (p *SQLitexStore) Delete(token string) error {
	conn, err := p.db.Take(context.Background())
	if err != nil {
		return err
	}
	defer p.db.Put(conn)

	err = sqlitex.Execute(conn, "DELETE FROM sessions WHERE token = $1",
		&sqlitex.ExecOptions{
			Args: []any{token},
		})
	return err
}

// All returns a map containing the token and data for all active (i.e.
// not expired) sessions in the SQLitexStore instance.
func (p *SQLitexStore) All() (map[string][]byte, error) {
	conn, err := p.db.Take(context.Background())
	if err != nil {
		return nil, err
	}
	defer p.db.Put(conn)

	sessions := make(map[string][]byte)

	err = sqlitex.Execute(conn, "SELECT token, data FROM sessions WHERE julianday('now') < expiry",
		&sqlitex.ExecOptions{
			ResultFunc: func(stmt *sqlite.Stmt) error {
				var data []byte
				var token = stmt.ColumnText(0)
				data = make([]byte, stmt.ColumnLen(1))
				stmt.ColumnBytes(1, data)
				sessions[token] = data
				return nil
			},
		})
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (p *SQLitexStore) startCleanup(interval time.Duration) {
	p.stopCleanup = make(chan bool)
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			err := p.deleteExpired()
			if err != nil {
				log.Println(err)
			}
		case <-p.stopCleanup:
			ticker.Stop()
			return
		}
	}
}

// StopCleanup terminates the background cleanup goroutine for the SQLitexStore
// instance. It's rare to terminate this; generally SQLitexStore instances and
// their cleanup goroutines are intended to be long-lived and run for the lifetime
// of your application.
//
// There may be occasions though when your use of the SQLitexStore is transient.
// An example is creating a new SQLitexStore instance in a test function. In this
// scenario, the cleanup goroutine (which will run forever) will prevent the
// SQLitexStore object from being garbage collected even after the test function
// has finished. You can prevent this by manually calling StopCleanup.
func (p *SQLitexStore) StopCleanup() {
	if p.stopCleanup != nil {
		p.stopCleanup <- true
	}
}

func (p *SQLitexStore) deleteExpired() error {
	conn, err := p.db.Take(context.Background())
	if err != nil {
		return err
	}
	defer p.db.Put(conn)

	return sqlitex.Execute(
		conn,
		"DELETE FROM sessions WHERE expiry < julianday('now')",
		nil,
	)
}
