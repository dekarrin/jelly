// Package dao provides data abstraction objects and database connection
// functions for the jellytest test server.
package dao

import "database/sql"

type Datastore struct {
	conn *sql.DB

	Messages Messages
}

func (ds Datastore) Close() error {
	var closeErr error

	if ds.conn != nil {
		closeErr = ds.conn.Close()
	}

	if closeErr != nil {
		return closeErr
	}

	ds.Messages.Close()

	return nil
}
