package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/internal/logging"
	mock_jelly "github.com/dekarrin/jelly/tools/mocks/jelly"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type testAPIConfig struct {
	jelly.CommonConfig
	Vriska int
}

func (cfg *testAPIConfig) FillDefaults() jelly.APIConfig {
	newCFG := new(testAPIConfig)
	*newCFG = *cfg

	newCFG.CommonConfig = newCFG.CommonConfig.FillDefaults().Common()

	return newCFG
}

func (cfg *testAPIConfig) Validate() error {
	if cfg.Vriska%8 != 0 {
		return errors.New("vriska must be a multiple of 8")
	}
	return nil
}

func (cfg *testAPIConfig) Common() jelly.CommonConfig {
	return cfg.CommonConfig
}

func (cfg *testAPIConfig) Set(name string, value interface{}) error {
	switch strings.ToLower(name) {
	case "vriska":
		v, err := jelly.TypedInt(name, value)
		if err == nil {
			cfg.Vriska = v
		}
		return err
	default:
		return cfg.CommonConfig.Set(name, value)
	}
}

func (cfg *testAPIConfig) SetFromString(name, value string) error {
	switch strings.ToLower(name) {
	case "vriska":
		if value == "" {
			return cfg.Set(name, 0)
		} else {
			if v, err := strconv.ParseInt(value, 10, 64); err != nil {
				return err
			} else {
				return cfg.Set(name, int(v))
			}
		}
	default:
		return cfg.CommonConfig.SetFromString(name, value)
	}
}

func (cfg *testAPIConfig) Get(name string) interface{} {
	switch strings.ToLower(name) {
	case "vriska":
		return cfg.Vriska
	default:
		return cfg.CommonConfig.Get(name)
	}
}

func (cfg *testAPIConfig) Keys() []string {
	keys := cfg.CommonConfig.Keys()
	keys = append(keys, "vriska")
	return keys
}

func Test_Add(t *testing.T) {
	getInitializedServer := func() *restServer {
		return &restServer{
			mtx:         &sync.Mutex{},
			apis:        map[string]jelly.API{},
			apiBases:    map[string]string{},
			basesToAPIs: map[string]string{},
			log:         logging.NoOpLogger{},
			dbs:         map[string]jelly.Store{},
			cfg:         jelly.Config{}.FillDefaults(),
		}
	}

	t.Run("add API with no config, nil routes", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)

		server := getInitializedServer()

		// execute
		err := server.Add("test", mockAPI)

		// assert
		assert.NoError(err)
		assert.Equal(mockAPI, server.apis["test"])
	})

	t.Run("add API with basic config, nil routes", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Authenticators().Return(nil)
		mockAPI.EXPECT().Init(gomock.Any()).Return(nil)

		server := getInitializedServer()
		server.cfg.APIs = map[string]jelly.APIConfig{
			"test": &testAPIConfig{
				CommonConfig: jelly.CommonConfig{
					Enabled: true,
				},
			},
		}

		// execute
		err := server.Add("test", mockAPI)

		// assert
		assert.NoError(err)
		assert.Equal(mockAPI, server.apis["test"])
	})
}

func Test_ServeForever_And_Shutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running tests that require server up")
	}
	getInitializedServer := func() *restServer {
		return &restServer{
			mtx:         &sync.Mutex{},
			apis:        map[string]jelly.API{},
			apiBases:    map[string]string{},
			basesToAPIs: map[string]string{},
			log:         logging.NoOpLogger{},
			dbs:         map[string]jelly.Store{},
			cfg:         jelly.Config{}.FillDefaults(),
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

		shutdownErr := server.http.Shutdown(timeLimitCtx)
		serveForeverErr := <-retErrChan

		// assert
		assert.NoError(shutdownErr)
		assert.ErrorIs(serveForeverErr, http.ErrServerClosed)
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

		shutdownErr := server.Shutdown(timeLimitCtx)
		serveForeverErr := <-retErrChan

		// assert
		assert.NoError(shutdownErr)
		assert.ErrorIs(serveForeverErr, http.ErrServerClosed)
	})

	t.Run("custom-api server, clean shutdown via Shutdown method", func(t *testing.T) {
		// setup
		assert := assert.New(t)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		rtr := chi.NewRouter()
		rtr.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Routes(gomock.Any()).Return(rtr)
		mockAPI.EXPECT().Shutdown(gomock.Any()).Return(nil)

		server := getInitializedServer()
		server.apis["test"] = mockAPI
		server.apiBases["test"] = "/test"
		server.cfg.APIs = map[string]jelly.APIConfig{
			"test": &testAPIConfig{
				CommonConfig: jelly.CommonConfig{
					Enabled: true,
				},
			},
		}
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

		shutdownErr := server.Shutdown(timeLimitCtx)
		serveForeverErr := <-retErrChan

		assert.NoError(shutdownErr)
		assert.ErrorIs(serveForeverErr, http.ErrServerClosed)
	})

	t.Run("custom-api server, api shutdown terminated by context", func(t *testing.T) {
		// setup
		assert := assert.New(t)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		rtr := chi.NewRouter()
		rtr.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Routes(gomock.Any()).Return(rtr)
		mockAPI.EXPECT().Shutdown(gomock.Any()).DoAndReturn(func(ctx context.Context) error {
			time.Sleep(10 * time.Second) // operation will take 10 seconds
			return nil
		})

		server := getInitializedServer()
		server.apis["test"] = mockAPI
		server.apiBases["test"] = "/test"
		server.cfg.APIs = map[string]jelly.APIConfig{
			"test": &testAPIConfig{
				CommonConfig: jelly.CommonConfig{
					Enabled: true,
				},
			},
		}
		retErrChan := make(chan error)

		// execute
		go func() {
			retErrChan <- server.ServeForever()
		}()

		// give it two seconds to come up (BAD BAD sleep synchronization. how about nosleep lib?)
		time.Sleep(1 * time.Second)

		// okay, shut it down with 2 seconds of grace time.
		timeLimitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		shutdownErr := server.Shutdown(timeLimitCtx)
		serveForeverErr := <-retErrChan

		assert.ErrorIs(shutdownErr, context.DeadlineExceeded)
		assert.ErrorIs(serveForeverErr, http.ErrServerClosed)
	})

	t.Run("custom-api server, api shutdown returns error", func(t *testing.T) {
		// setup
		assert := assert.New(t)

		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		rtr := chi.NewRouter()
		rtr.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Routes(gomock.Any()).Return(rtr)
		mockAPI.EXPECT().Shutdown(gomock.Any()).Return(errors.New("shutdown error"))

		server := getInitializedServer()
		server.apis["test"] = mockAPI
		server.apiBases["test"] = "/test"
		server.cfg.APIs = map[string]jelly.APIConfig{
			"test": &testAPIConfig{
				CommonConfig: jelly.CommonConfig{
					Enabled: true,
				},
			},
		}
		retErrChan := make(chan error)

		// execute
		go func() {
			retErrChan <- server.ServeForever()
		}()

		// give it two seconds to come up (BAD BAD sleep synchronization. how about nosleep lib?)
		time.Sleep(1 * time.Second)

		// okay, shut it down with 2 seconds of grace time.
		timeLimitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		shutdownErr := server.Shutdown(timeLimitCtx)
		serveForeverErr := <-retErrChan

		assert.ErrorContains(shutdownErr, "shutdown error")
		assert.ErrorIs(serveForeverErr, http.ErrServerClosed)
	})
}
