package jelly

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/dekarrin/jelly/config"
	"github.com/dekarrin/jelly/db"
	"github.com/dekarrin/jelly/logging"
	"github.com/stretchr/testify/assert"
)

func Test_ServeForever(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running tests that require server up")
	}
	getInitializedServer := func() *RESTServer {
		return &RESTServer{
			mtx:         &sync.Mutex{},
			apis:        map[string]API{},
			apiBases:    map[string]string{},
			basesToAPIs: map[string]string{},
			log:         logging.NoOpLogger{},
			dbs:         map[string]db.Store{},
			cfg:         config.Config{}.FillDefaults(),
		}
	}

	t.Run("empty server, clean shutdown via *http.Server stop", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		server := getInitializedServer()
		retErrChan := make(chan error)

		// execute
		go func() {
			retErrChan <- server.ServeForever()
		}()

		// give it two seconds to come up (BAD BAD sleep synchronization. how about nosleep lib?)
		time.Sleep(1 * time.Second)

		// okay, shut it down with 10 seconds of grace time.
		timeLimitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		server.http.Shutdown(timeLimitCtx)
		serveForeverError := <-retErrChan

		// assert
		assert.ErrorIs(serveForeverError, http.ErrServerClosed)
	})

	t.Run("empty server, clean shutdown via Shutdown method", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		server := getInitializedServer()
		retErrChan := make(chan error)

		// execute
		go func() {
			retErrChan <- server.ServeForever()
		}()

		// give it two seconds to come up (BAD BAD sleep synchronization. how about nosleep lib?)
		time.Sleep(1 * time.Second)

		// okay, shut it down with 10 seconds of grace time.
		timeLimitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		server.Shutdown(timeLimitCtx)
		serveForeverError := <-retErrChan

		// assert
		assert.ErrorIs(serveForeverError, http.ErrServerClosed)
	})
}
