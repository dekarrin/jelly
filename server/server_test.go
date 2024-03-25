package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
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

func Test_restServer_Config(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		server := getInitializedServer()

		// execute
		cfg := server.Config()

		// assert
		assert.Equal(server.cfg, cfg)
	})

	t.Run("empty server", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		server := &restServer{}

		// execute
		cfg := server.Config()

		// assert
		assert.Equal(server.cfg.FillDefaults(), cfg)
	})
}

func Test_restServer_RoutesIndex(t *testing.T) {
	type apiAndConf struct {
		api jelly.API
		cfg jelly.APIConfig
	}

	testCases := []struct {
		name          string
		serverSetup   func() *restServer
		apiMocksSetup func(t *testing.T) map[string]apiAndConf
		expect        string
		expectErr     string
	}{
		{
			name:        "empty server",
			serverSetup: getInitializedServer,
			expect:      "(no routes added)",
		},
		{
			name:        "server with one route",
			serverSetup: getInitializedServer,
			apiMocksSetup: func(t *testing.T) map[string]apiAndConf {
				mockCtrl := gomock.NewController(t)
				t.Cleanup(mockCtrl.Finish)

				rtr := chi.NewRouter()
				rtr.Get("/test", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				mockAPI := mock_jelly.NewMockAPI(mockCtrl)
				mockAPI.EXPECT().Routes(gomock.Any()).Return(rtr)

				cfg := (&testAPIConfig{
					CommonConfig: jelly.CommonConfig{
						Enabled: true,
					},
				}).FillDefaults()

				return map[string]apiAndConf{
					"test": {
						api: mockAPI,
						cfg: cfg,
					},
				}
			},
			expect: "* /test - GET",
		},
		{
			name:        "empty server",
			serverSetup: getInitializedServer,
			expect:      "(no routes added)",
		},
		{
			name:        "server with serveral APIs and routes",
			serverSetup: getInitializedServer,
			apiMocksSetup: func(t *testing.T) map[string]apiAndConf {
				mockCtrl := gomock.NewController(t)
				t.Cleanup(mockCtrl.Finish)

				rtr1 := chi.NewRouter()
				rtr1.Get("/test", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				mockAPI1 := mock_jelly.NewMockAPI(mockCtrl)
				mockAPI1.EXPECT().Routes(gomock.Any()).Return(rtr1)

				cfg1 := (&testAPIConfig{
					CommonConfig: jelly.CommonConfig{
						Name:    "test1",
						Enabled: true,
					},
				}).FillDefaults()

				rtr2 := chi.NewRouter()
				rtr2.Get("/test", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				mockAPI2 := mock_jelly.NewMockAPI(mockCtrl)
				mockAPI2.EXPECT().Routes(gomock.Any()).Return(rtr2)

				cfg2 := (&testAPIConfig{
					CommonConfig: jelly.CommonConfig{
						Name:    "test2",
						Enabled: true,
						Base:    "/api2/",
					},
				}).FillDefaults()

				rtr3 := chi.NewRouter()
				rtr3.Get("/echo", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
				rtr3.Post("/hello", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})
				rtr3.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				mockAPI3 := mock_jelly.NewMockAPI(mockCtrl)
				mockAPI3.EXPECT().Routes(gomock.Any()).Return(rtr3)

				cfg3 := (&testAPIConfig{
					CommonConfig: jelly.CommonConfig{
						Name:    "autoreply",
						Enabled: true,
						Base:    "/autoreply",
					},
				}).FillDefaults()

				return map[string]apiAndConf{
					"test1": {
						api: mockAPI1,
						cfg: cfg1,
					},
					"test2": {
						api: mockAPI2,
						cfg: cfg2,
					},
					"autoreplier": {
						api: mockAPI3,
						cfg: cfg3,
					},
				}
			},
			expect: "* /api2/test - GET\n" +
				"* /autoreply/echo - GET\n" +
				"* /autoreply/hello - GET, POST\n" +
				"* /test - GET",
		},
		{
			name:        "routes panics",
			serverSetup: getInitializedServer,
			apiMocksSetup: func(t *testing.T) map[string]apiAndConf {
				mockCtrl := gomock.NewController(t)
				t.Cleanup(mockCtrl.Finish)

				rtr := chi.NewRouter()
				rtr.Get("/test", func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				})

				mockAPI := mock_jelly.NewMockAPI(mockCtrl)
				mockAPI.EXPECT().Routes(gomock.Any()).DoAndReturn(func(_ interface{}) error {
					panic("my special panic")
				})

				cfg := (&testAPIConfig{
					CommonConfig: jelly.CommonConfig{
						Name:    "test1",
						Enabled: true,
					},
				}).FillDefaults()

				return map[string]apiAndConf{
					"test": {
						api: mockAPI,
						cfg: cfg,
					},
				}
			},
			expectErr: "my special panic",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// setup
			assert := assert.New(t)
			server := tc.serverSetup()

			if tc.apiMocksSetup != nil {
				server.cfg.APIs = map[string]jelly.APIConfig{}

				mocks := tc.apiMocksSetup(t)

				for apiName, mockAPI := range mocks {
					server.apis[apiName] = mockAPI.api
					server.cfg.APIs[apiName] = mockAPI.cfg
					server.apiBases[apiName] = mockAPI.cfg.Common().Base
				}
			}

			// execute
			rtr, err := server.RoutesIndex()

			// assert
			if tc.expectErr != "" {
				assert.ErrorContains(err, tc.expectErr)
			} else {
				assert.NoError(err)
				assert.Equal(tc.expect, rtr.FormattedList())
			}
		})
	}
}

func Test_restServer_Add(t *testing.T) {
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

	t.Run("add API with basic config", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Init(gomock.Any()).Return(nil)
		mockAPI.EXPECT().Authenticators().Return(nil)

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

	t.Run("add API with authenticators", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Init(gomock.Any()).Return(nil)
		mockAPI.EXPECT().Authenticators().Return(map[string]jelly.Authenticator{"jwt": nil})

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

	t.Run("add API whose Init() fails", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Init(gomock.Any()).Return(errors.New("init error"))

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
		assert.ErrorContains(err, "init error")
	})

	t.Run("add API whose Init() panics", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Init(gomock.Any()).Do(func(_ interface{}) {
			panic("my special panic")
		})

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
		assert.ErrorContains(err, "my special panic")
	})

	t.Run("add API whose Authenticators() panics", func(t *testing.T) {
		// setup
		assert := assert.New(t)
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Init(gomock.Any()).Return(nil)
		mockAPI.EXPECT().Authenticators().DoAndReturn(func() {
			panic("my special panic")
		})

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
		assert.ErrorContains(err, "my special panic")
	})
}

func Test_restServer_ServeForever_And_Shutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running tests that require server up")
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

		mockCtrl := newGoroutineAwareMockCtrl(t)
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

		mockCtrl := newGoroutineAwareMockCtrl(t)
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

		mockCtrl := newGoroutineAwareMockCtrl(t)
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

	t.Run("custom-api server, api shutdown panics", func(t *testing.T) {
		// setup
		assert := assert.New(t)

		mockCtrl := newGoroutineAwareMockCtrl(t)
		defer mockCtrl.Finish()

		rtr := chi.NewRouter()
		rtr.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Routes(gomock.Any()).Return(rtr)
		mockAPI.EXPECT().Shutdown(gomock.Any()).Do(func(ctx context.Context) {
			panic("my special panic")
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

		assert.ErrorContains(shutdownErr, "my special panic")
		assert.ErrorIs(serveForeverErr, http.ErrServerClosed)
	})

	t.Run("custom-api server, routes panics", func(t *testing.T) {
		// setup
		assert := assert.New(t)

		mockCtrl := newGoroutineAwareMockCtrl(t)
		defer mockCtrl.Finish()

		rtr := chi.NewRouter()
		rtr.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		mockAPI := mock_jelly.NewMockAPI(mockCtrl)
		mockAPI.EXPECT().Routes(gomock.Any()).DoAndReturn(func(_ interface{}) error {
			panic("my special panic")
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

		assert.ErrorContains(shutdownErr, "server is not running")
		assert.ErrorContains(serveForeverErr, "my special panic")
	})
}

func newGoroutineAwareMockCtrl(t *testing.T) *gomock.Controller {
	return gomock.NewController(newGoroutineAwareReporter(t))
}

type goroutineAwareReporter struct {
	t       *testing.T
	creator string
}

func newGoroutineAwareReporter(t *testing.T) *goroutineAwareReporter {
	st := string(debug.Stack())
	var creator string

	lines := strings.Split(st, "\n")
	for lineno := range lines {
		if strings.Contains(lines[lineno], "newGoroutineCheckReporter") {
			// the first time we encounter newGoroutineCheckReporter, we're at
			// this function in the stack. skip the next line as it
			// is the file and line number of this call.
			if lineno+2 < len(lines) {
				creator = lines[lineno+2]
			}
			continue
		}
	}

	return &goroutineAwareReporter{t: t, creator: creator}
}

func (r *goroutineAwareReporter) Helper() {
	r.t.Helper()
}

func (r *goroutineAwareReporter) Errorf(format string, args ...interface{}) {
	r.t.Errorf(format, args...)
}

func (r *goroutineAwareReporter) Fatalf(format string, args ...interface{}) {
	r.t.Helper()

	st := string(debug.Stack())

	var inCreatorRoutine bool

	lines := strings.Split(st, "\n")
	for lineno := range lines {
		if lines[lineno] == r.creator {
			// if we hit the exact line, the original caller is in the stack
			inCreatorRoutine = true
			break
		}
	}

	if inCreatorRoutine {
		r.t.Fatalf(format, args...)
	} else {
		r.t.Errorf("FATAL ERROR FROM OTHER ROUTINE: %s", fmt.Sprintf(format, args...))
	}
}

func getInitializedServer() *restServer {
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
