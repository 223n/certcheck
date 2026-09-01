// Command certcheck reports how long the TLS certificates of the configured
// endpoints remain valid, and notifies Slack about the ones that are about to
// expire.
//
// Usage:
//
//	certcheck [-c config.yml]
//	certcheck -v
//
// See README.md for the configuration file format and docs/ARCHITECTURE.md for
// how the packages fit together.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/223n/certcheck/internal/checker"
	"github.com/223n/certcheck/internal/config"
	"github.com/223n/certcheck/internal/notify"
	"github.com/223n/certcheck/internal/runner"
)

// Version and Revision are injected at build time with -ldflags -X.
var (
	// Version is the certcheck version, for example v1.3.0.
	Version = "{version}"
	// Revision is the commit the binary was built from.
	Revision = "{revision}"
)

// defaultConfigFile is used when -c is not given.
const defaultConfigFile = "certcheck.yml"

func main() {
	if err := run(os.Args[1:], log.Default()); err != nil {
		log.Fatal(err)
	}
}

// run is main without the process exit, so that it can be tested.
func run(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("certcheck", flag.ContinueOnError)
	filename := fs.String("c", defaultConfigFile, "config file name")
	shortVersion := fs.Bool("v", false, "prints current version")
	longVersion := fs.Bool("version", false, "prints current version")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *shortVersion || *longVersion {
		logger.Print(version())
		return nil
	}

	cfg, err := config.Load(*filename)
	if err != nil {
		return err
	}

	runner.New(cfg, checker.New(), notify.NewSlack(), logger).Run(context.Background())
	return nil
}

// version renders the build information printed by -v.
func version() string {
	return fmt.Sprintf("%s %s/%s, build %s", Version, runtime.GOOS, runtime.GOARCH, Revision)
}
