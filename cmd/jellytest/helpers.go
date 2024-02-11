package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/cmd/jellytest/dao"
	"github.com/dekarrin/jelly/logging"
	"github.com/google/uuid"
)

func initDBWithMessages(ctx context.Context, log logging.Logger, repo dao.Templates, creator uuid.UUID, msgs []string) error {
	for _, m := range msgs {
		dbMsg := dao.Template{
			Content: m,
			Creator: creator,
		}
		created, err := repo.Create(ctx, dbMsg)
		if err != nil {
			if !errors.Is(err, jelly.DBErrConstraintViolation) {
				return fmt.Errorf("create initial messages: %w", err)
			} else {
				log.Tracef("Skipping adding message to DB via config; already exists: %q", m)
			}
		} else {
			log.Debugf("Added new message to DB via config: %s - %q", created.ID, created.Content)
		}
	}
	return nil
}

// Template is the representation of a message template resource.
type Template struct {
	ID      string `json:"id,omitempty"`
	Content string `json:"content"`
	Creator string `json:"creator,omitempty"`
	Path    string `json:"path,omitempty"`
}

// DAO creates a data abstraction object that represents this model. Conversion
// of values is performed; while empty values are allowed for ID and Creator
// (and will simply result in a zero-value ID in the returned object), non-empty
// invalid values will cause an error.
func (m Template) DAO() (dao.Template, error) {
	var err error

	t := dao.Template{
		Content: m.Content,
	}

	if m.ID != "" {
		t.ID, err = uuid.Parse(m.ID)
		if err != nil {
			return t, err
		}
	}
	if m.Creator != "" {
		t.Creator, err = uuid.Parse(m.Creator)
		if err != nil {
			return t, err
		}
	}

	return t, nil
}

func (t Template) Validate(requireFormatVerb bool) error {
	if t.Content == "" {
		return errors.New("'content' field must exist and be set to a non-empty value")
	}

	if requireFormatVerb {
		if !strings.Contains("%s", t.Content) && !strings.Contains("%v", t.Content) && !strings.Contains("%q", t.Content) {
			return errors.New("template must contain at least one %v, %s, or %q")
		}
	}

	return nil
}

func daoToTemplates(ts []dao.Template, uriBase string) []Template {
	output := make([]Template, len(ts))
	for i := range ts {
		output[i] = daoToTemplate(ts[i], uriBase)
	}
	return output
}

func daoToTemplate(t dao.Template, uriBase string) Template {
	m := Template{
		Content: t.Content,
		Path:    fmt.Sprintf("%s/templates/%s", uriBase, t.ID),
	}

	var zeroUUID uuid.UUID

	if t.ID != zeroUUID {
		m.ID = t.ID.String()
	}
	if t.Creator != zeroUUID {
		m.Creator = t.Creator.String()
	}

	return m
}
