package sqlite

import (
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/dekarrin/jelly/config"
	jellydao "github.com/dekarrin/jelly/dao"
)

func New(cfg config.Database) (jellydao.Store, error) {

	ds := dao.Datastore{}

	return ds, nil
}
