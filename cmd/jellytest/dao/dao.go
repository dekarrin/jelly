// Package dao provides data abstraction objects and database connection
// functions for the jellytest test server.
package dao

import "database/sql"

type Datastore struct {
	DB *sql.DB

	NiceMessages   Messages
	RudeMessages   Messages
	SecretMessages Messages
	EchoMessages   Messages
}

func (ds Datastore) Close() error {
	var closeErr error

	if ds.DB != nil {
		closeErr = ds.DB.Close()
	}

	if closeErr != nil {
		return closeErr
	}

	ds.NiceMessages.Close()
	ds.RudeMessages.Close()
	ds.SecretMessages.Close()
	ds.EchoMessages.Close()

	return nil
}
