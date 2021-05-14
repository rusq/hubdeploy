package main

import (
	"flag"
	"os"
	"path/filepath"

	"github.com/rusq/hubdeploy/internal/hookers"

	"github.com/rusq/dlog"
	"github.com/rusq/hubdeploy/internal/deploysrv"
	"github.com/rusq/osenv"
)

var (
	port    = flag.String("p", osenv.String("PORT", "16991"), "http server `port`")
	host    = flag.String("host", osenv.String("HOST", "127.0.0.1"), "`host or ip` to bind to")
	prefix  = flag.String("prefix", osenv.String("PREFIX", "/"), "api path prefix")
	cert    = flag.String("cert", "", "certificate path")
	key     = flag.String("key", "", "certificate key")
	config  = flag.String("c", osenv.String("CONFIG_YAML", "hubdeploy.yml"), "config `file`")
	verbose = flag.Bool("v", false, "verbose output")
	log     = flag.String("l", "", "log `file` or device")
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
	dlog.Println("starting up...")
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

	addr := *host + ":" + *port
	dlog.Println("listening on", addr)
	if err := srv.ListenAndServe(addr); err != nil {
		dlog.Fatal(err)
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
		dlog.Println(err)
		return err
	}
	dlog.SetOutput(f)
	return nil
}
