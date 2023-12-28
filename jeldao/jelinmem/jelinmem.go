// Package jelinmem provides an in-memory database for use with model types.
package jelinmem

import (
	"fmt"

	"github.com/dekarrin/jelly/jeldao"
)

type store struct {
	repos map[string]any
}

func NewDatastore() jeldao.Store {
	st := &store{
		users:  NewUsersRepository(),
		regs:   NewRegistrationsRepository(),
		games:  NewGamesRepository(),
		gd:     NewGameDatasRepository(),
		seshes: NewSessionsRepository(),
	}
	st.coms = NewCommandsRepository(st.seshes)
	return st
}

func (s *store) Users() jeldao.UserRepository {
	return s.users
}

func (s *store) Registrations() jeldao.RegistrationRepository {
	return s.regs
}

func (s *store) Games() jeldao.GameRepository {
	return s.games
}

func (s *store) GameData() jeldao.GameDataRepository {
	return s.gd
}

func (s *store) Sessions() jeldao.SessionRepository {
	return s.seshes
}

func (s *store) Commands() jeldao.CommandRepository {
	return s.coms
}

func (s *store) Close() error {
	var err error
	var nextErr error

	nextErr = s.users.Close()
	if nextErr != err {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally, %w", err, nextErr)
		} else {
			err = nextErr
		}
	}
	nextErr = s.regs.Close()
	if nextErr != err {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally, %w", err, nextErr)
		} else {
			err = nextErr
		}
	}
	nextErr = s.games.Close()
	if nextErr != err {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally, %w", err, nextErr)
		} else {
			err = nextErr
		}
	}
	nextErr = s.gd.Close()
	if nextErr != err {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally, %w", err, nextErr)
		} else {
			err = nextErr
		}
	}
	nextErr = s.seshes.Close()
	if nextErr != err {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally, %w", err, nextErr)
		} else {
			err = nextErr
		}
	}

	return err
}
