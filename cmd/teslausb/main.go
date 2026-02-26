package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/teslausb-go/teslausb/internal/config"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/mutable/teslausb/config.yaml", "config file path")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to load config: %v", err)
	}
	if cfg == nil {
		cfg = &config.Config{}
	}

	log.Printf("teslausb %s starting", version)
	_ = cfg
}
