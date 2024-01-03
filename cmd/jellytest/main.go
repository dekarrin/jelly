/*
Jellytest starts a jelly-based RESTServer that uses the pre-rolled jelly auth
API as well as its own separate echo API.

Usage:

	jellytest [flags]

Once started, the server will listen for HTTP requests and respond to them as
configured. The main endpoints of interest are:

  - /echo - request a reply with what the user said
  - /hello/nice - requests a polite greeting
  - /hello/rude - requests a rude greeting
  - /hello/random - requests a random greeting, either nice or rude
  - /hello/secret - requests a secret greeting that requires login

Additionally, the jelly auth API is started with its endpoints under /auth under
the base URI for the server, if one is configured.

The flags are:

	-c, --conf PATH
		Use the given file for the configuration instead of './jelly.yml'. The
		file must be in JSON or YAML format.
*/
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/dekarrin/jellog"
	"github.com/dekarrin/jelly"
	"github.com/dekarrin/jelly/config"
	"github.com/spf13/pflag"
)

const (
	exitSuccess   = 0
	exitError     = 1
	exitPanic     = 2
	exitInterrupt = 3
)

var exitCode int

var (
	flagConf = pflag.StringP("config", "c", "jelly.yml", "Path to configuration file")
)

func main() {
	// context for signal handling. might be overkill, taking this from example
	// located at https://pace.dev/blog/2020/02/17/repond-to-ctrl-c-interrupt-signals-gracefully-with-context-in-golang-by-mat-ryer.html
	ctx := context.Background()
	ctx, cancelMainContext := context.WithCancel(ctx)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	defer func() {
		signal.Stop(signalChan)
		cancelMainContext()
	}()
	// listen for signals
	go func() {
		select {
		case <-signalChan: // first signal, cancel context
			cancelMainContext()
		case <-ctx.Done():
		}

		<-signalChan // second signal, hard exit
		os.Exit(exitInterrupt)
	}()

	defer func() {
		if panicErr := recover(); panicErr != nil {
			fmt.Fprintf(os.Stderr, "fatal panic: %v\n", panicErr)
			exitCode = exitPanic
		}
		os.Exit(exitCode)
	}()

	pflag.Parse()

	stdErrOutput := jellog.NewStderrHandler(nil)
	logger := jellog.New(jellog.Defaults[string]().
		WithComponent("jelly"))
	logger.AddHandler(jellog.LvTrace, stdErrOutput)

	logger.Infof("Loading config file %s...", *flagConf)
	conf, err := config.Load(*flagConf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		exitCode = exitError
		return
	}

	server, err := jelly.New(&conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err.Error())
		exitCode = exitError
		return
	}

	logger.Info("Starting server...")

	go func() {
		err := server.ServeForever()
		if errors.Is(err, http.ErrServerClosed) {
			logger.Info("Server shutdown by request")
		} else {
			logger.Errorf("Server encountered a problem: %v", err)
		}
	}()

	logger.Info("Jelly test server started; Ctrl-C (SIGINT) to stop")

	// wait forever, checking for interrupt and doing clean shutdown if we get
	// it
	for {
		select {
		case <-ctx.Done():
			// cleanup

			// ctrl-C likes to write "^C" or similar in some console output, so
			// insert a break right after that. This is not cross-platform; if
			// an indication of ctrl C is not written, there may be an awkward
			// break in stderr, but at least we tried.
			logger.InsertBreak(jellog.LvAll)

			logger.Info("SIGINT received; cleaning up server...")
			err := server.Shutdown(context.Background())
			if err != nil {
				logger.Warn(err.Error())
			}
			logger.Info("Server shutdown complete")
			return
		default:
			// just spinlock for a sec
			time.Sleep(100 * time.Millisecond)
		}
	}

}
