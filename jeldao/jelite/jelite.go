// Package jelite provides an interface into SQLite compatible with the rest of
// jelly packages.
package jelite

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"path/filepath"
	"time"

	"github.com/dekarrin/rezi"
	"github.com/dekarrin/tunaq/internal/game"
	"github.com/dekarrin/tunaq/server/dao"
	"github.com/dekarrin/tunaq/server/serr"
	"github.com/google/uuid"
	"modernc.org/sqlite"
)

type Store struct {
	dbFilename string

	db *sql.DB
}

type AuthUserStore struct {
	Store
}

func NewDatastore(storageDir string) (dao.Store, error) {
	st := &store{
		dbFilename:         "data.db",
		gameDataDBFilename: "worlds.db",
	}

	fileName := filepath.Join(storageDir, st.dbFilename)
	worldFileName := filepath.Join(storageDir, st.gameDataDBFilename)

	var err error
	st.db, err = sql.Open("sqlite", fileName)
	if err != nil {
		return nil, wrapDBError(err)
	}
	st.gameDataDB, err = sql.Open("sqlite", worldFileName)
	if err != nil {
		return nil, wrapDBError(err)
	}

	st.gd = &GameDatasDB{db: st.gameDataDB}
	st.gd.init()

	st.users = &UsersDB{db: st.db}
	st.users.init()

	st.regs = &RegistrationsDB{db: st.db}
	st.regs.init(true)

	st.games = &GamesDB{db: st.db}
	st.games.init(true)

	st.seshes = &SessionsDB{db: st.db}
	st.seshes.init(true)

	st.coms = &CommandsDB{db: st.db}
	st.coms.init(true)

	return st, nil
}

func (s *store) Users() dao.UserRepository {
	return s.users
}

func (s *store) Registrations() dao.RegistrationRepository {
	return s.regs
}

func (s *store) Games() dao.GameRepository {
	return s.games
}

func (s *store) GameData() dao.GameDataRepository {
	return s.gd
}

func (s *store) Sessions() dao.SessionRepository {
	return s.seshes
}

func (s *store) Commands() dao.CommandRepository {
	return s.coms
}

func (s *store) Close() error {
	worldsDBErr := s.gameDataDB.Close()
	mainDBErr := s.db.Close()

	var err error
	if worldsDBErr != nil {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally: %s: %w", err.Error(), s.gameDataDBFilename, worldsDBErr)
		} else {
			err = fmt.Errorf("%s: %w", s.gameDataDBFilename, worldsDBErr)
		}
	}
	if mainDBErr != nil {
		if err != nil {
			err = fmt.Errorf("%s\nadditionally: %s: %w", err.Error(), s.dbFilename, mainDBErr)
		} else {
			err = fmt.Errorf("%s: %w", s.dbFilename, err)
		}
	}
	return err
}

// convertToDB_Role converts a dao.Role to storage DB format.
func convertToDB_Role(r dao.Role) string {
	return r.String()
}

// convertToDB_Email converts a *mail.Address to storage DB format. If the
// pointer is nil, it will return the zero value.
func convertToDB_Email(email *mail.Address) string {
	if email == nil {
		return ""
	}
	return email.Address
}

// convertToDB_UUID converts a uuid.UUID to storage DB format on disk.
func convertToDB_UUID(u uuid.UUID) string {
	return u.String()
}

// convertToDB_Time converts a time.Time to storage DB format on disk.
func convertToDB_Time(t time.Time) int64 {
	return t.Unix()
}

// convertToDB_ByteSlice converts bytes to storage DB format on disk.
func convertToDB_ByteSlice(b []byte) string {
	if len(b) < 1 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// convertToDBBytes converts a *game.State to storage DB format on disk. If the
// pointer is nil, it will return the zero value.
func convertToDB_GameStatePtr(g *game.State) string {
	if g == nil {
		return ""
	}

	// first get the rezi-encoded bytes
	stateData := rezi.EncBinary(g)
	return convertToDB_ByteSlice(stateData)
}

// convertFromDB_Email converts storage DB format value to a *mail.Address
// and stores it at the address pointed to by target. If the zero value is
// provided, target is set to a nil pointer. If there is a problem with the
// decoding, the returned error will be of type serr.Error, and will wrap
// dao.ErrDecodingFailure. If this function returns a non-nil error, target will
// not have been modified.
func convertFromDB_Email(s string, target **mail.Address) error {
	if s == "" {
		*target = nil
		return nil
	}

	email, err := mail.ParseAddress(s)
	if err != nil {
		return serr.New("", err, dao.ErrDecodingFailure)
	}

	*target = email
	return nil
}

// convertFromDB_Role converts storage DB format value to a dao.Role and
// stores it at the address pointed to by target. If there is a problem with the
// decoding, the returned error will be of type serr.Error, and will wrap
// dao.ErrDecodingFailure. If this function returns a non-nil error, target will
// not have been modified.
func convertFromDB_Role(s string, target *dao.Role) error {
	r, err := dao.ParseRole(s)
	if err != nil {
		return serr.New("", err, dao.ErrDecodingFailure)
	}
	*target = r
	return nil
}

// convertFromDB_UUID converts storage DB format value to a uuid.UUID and
// stores it at the address pointed to by target. If there is a problem with the
// decoding, the returned error will be of type serr.Error, and will wrap
// dao.ErrDecodingFailure. If this function returns a non-nil error, target will
// not have been modified.
func convertFromDB_UUID(s string, target *uuid.UUID) error {
	u, err := uuid.Parse(s)
	if err != nil {
		return serr.New("", err, dao.ErrDecodingFailure)
	}
	*target = u
	return nil
}

// convertFromDB_Bytes converts storage DB format value to a time.Time and
// stores it at the address pointed to by target. If there is a problem with the
// decoding, the returned error will be of type serr.Error, and will wrap
// dao.ErrDecodingFailure. If this function returns a non-nil error, target will
// not have been modified.
func convertFromDB_Time(i int64, target *time.Time) error {
	t := time.Unix(i, 0)
	*target = t
	return nil
}

// convertFromDB_Bytes converts storage DB format string to an actual byte
// slice and stores it at the address pointed to by target. If there is a
// problem with the decoding, the returned error will be of type serr.Error, and
// will wrap dao.ErrDecodingFailure. If this function returns a non-nil error,
// target will not have been modified.
func convertFromDB_ByteSlice(s string, target *[]byte) error {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return serr.New("", err, dao.ErrDecodingFailure)
	}
	*target = decoded
	return nil
}

// convertFromDB_GameStatePtr converts a storage DB format string to an actual
// game state pointer and stores it at the address pointed to by target. If the
// zero value is provided, target is set to a nil pointer. If there is a problem
// with the decoding, the returned error will be of type serr.Error, and will
// wrap dao.ErrDecodingFailure. If this function returns a non-nil error, target
// will not have been modified.
func convertFromDB_GameStatePtr(s string, target **game.State) error {
	if s == "" {
		*target = nil
		return nil
	}

	// first, need to get a byte slice
	var stateData []byte
	err := convertFromDB_ByteSlice(s, &stateData)
	if err != nil {
		return serr.New("decode stored to bytes", err)
	}

	g := &game.State{}
	n, err := rezi.DecBinary(stateData, g)
	if err != nil {
		return serr.New("REZI decode: %w", err, dao.ErrDecodingFailure)
	}
	if n != len(stateData) {
		return serr.New(fmt.Sprintf("REZI decoded byte count mismatch; only consumed %d/%d bytes", n, len(stateData)), dao.ErrDecodingFailure)
	}

	*target = g
	return nil
}

func wrapDBError(err error) error {
	sqliteErr := &sqlite.Error{}
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code() == 19 {
			return dao.ErrConstraintViolation
		}
		return fmt.Errorf("%s", sqlite.ErrorCodeString[sqliteErr.Code()])
	} else if errors.Is(err, sql.ErrNoRows) {
		return dao.ErrNotFound
	}
	return err
}
