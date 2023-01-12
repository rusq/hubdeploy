package main

import (
	"errors"
	"flag"
	"os"
	"path/filepath"

	"github.com/rusq/dlog"
	"github.com/rusq/gotsr"
	"github.com/rusq/osenv"

	"github.com/rusq/hubdeploy/internal/deploysrv"
	"github.com/rusq/hubdeploy/internal/hookers"
)

var (
	port    = flag.String("p", osenv.String("PORT", "9999"), "http server `port`")
	host    = flag.String("host", osenv.String("HOST", "127.0.0.1"), "`host or ip` to bind to")
	prefix  = flag.String("prefix", osenv.String("PREFIX", "/"), "api path prefix")
	cert    = flag.String("cert", "", "certificate path")
	key     = flag.String("key", "", "certificate key")
	config  = flag.String("c", osenv.String("CONFIG_YAML", "hubdeploy.yml"), "config `file`")
	verbose = flag.Bool("v", false, "verbose output")
	log     = flag.String("l", "", "log `file` or device")
	stop    = flag.Bool("stop", false, "stops the process")
)

func main() {
	flag.Parse()
	if *port == "" {
		flag.Usage()
		dlog.Fatal("invalid port")
	}
	if *config == "" {
		flag.Usage()
		dlog.Fatal("no config file")
	}
	p, err := gotsr.New()
	if err != nil {
		dlog.Fatal(err)
	}
	if *stop {
		if err := p.Terminate(); err != nil {
			if errors.Is(err, gotsr.ErrNotRunning) {
				dlog.Println(err)
				return
			}
			dlog.Fatal(err)
		}
		dlog.Println("stopped")
		return
	}
	if running, err := p.IsRunning(); err != nil {
		dlog.Fatal(err)
	} else if running {
		dlog.Fatal("already running")
	}

	headless, err := p.TSR()
	if err != nil {
		dlog.Fatal(err)
	}

	addr := *host + ":" + *port
	if !headless {
		dlog.Println("starting up...")
		dlog.Println("listening on", addr)
		return
	} else {
		dlog.SetDebug(*verbose)
		cfg, err := deploysrv.LoadConfig(*config)
		if err != nil {
			dlog.Fatal(err)
		}
		if err := deploysrv.Register(new(hookers.DockerHub)); err != nil {
			dlog.Fatal(err)
		}
		srv, err := deploysrv.New(cfg, deploysrv.OptWithCert(*cert, *key), deploysrv.OptWithPrefix(*prefix))
		if err != nil {
			dlog.Fatal(err)
		}
		if err := initlog(*log); err != nil {
			dlog.Fatal(err)
		}

		dlog.Println("listening on", addr)
		if err := srv.ListenAndServe(addr); err != nil {
			dlog.Fatal(err)
		}
	}
}

func initlog(filename string) error {
	if filename == "-" {
		dlog.SetOutput(os.Stderr)
		return nil
	} else if filename == "" {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		base := filepath.Base(exe)
		ext := filepath.Ext(base)
		filename = base[:len(base)-len(ext)] + ".log"
	}

	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	dlog.SetOutput(f)
	return nil
}
