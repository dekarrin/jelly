// Package dao provides data abstraction objects and database connection
// functions for the jellytest test server.
package dao

import "database/sql"

type Datastore struct {
	DB *sql.DB

	NiceTemplates   Templates
	RudeTemplates   Templates
	SecretTemplates Templates
	EchoTemplates   Templates
}

func (ds Datastore) Close() error {
	var closeErr error

	if ds.DB != nil {
		closeErr = ds.DB.Close()
	}

	if closeErr != nil {
		return closeErr
	}

	ds.NiceTemplates.Close()
	ds.RudeTemplates.Close()
	ds.SecretTemplates.Close()
	ds.EchoTemplates.Close()

	return nil
}
