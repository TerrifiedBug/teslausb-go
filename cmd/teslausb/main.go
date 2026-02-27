package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/teslausb-go/teslausb/internal/config"
	"github.com/teslausb-go/teslausb/internal/monitor"
	"github.com/teslausb-go/teslausb/internal/state"
	"github.com/teslausb-go/teslausb/internal/system"
	"github.com/teslausb-go/teslausb/internal/web"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "/mutable/teslausb/config.yaml", "config file path")
	listenAddr := flag.String("addr", ":80", "web server listen address")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// Process lock
	lockFile, err := os.OpenFile("/var/run/teslausb.lock", os.O_CREATE|os.O_RDWR, 0644)
	if err == nil {
		if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			log.Fatal("another instance is running")
		}
		defer lockFile.Close()
	}

	log.Printf("teslausb %s starting", version)

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("config load warning: %v", err)
	}
	if cfg == nil {
		config.Save(*configPath, &config.Config{
			Temperature: config.Temperature{WarningCelsius: 70, CautionCelsius: 60},
		})
		config.Load(*configPath)
	}

	// Apply system tuning
	system.ApplyTuning()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	// Start monitors
	go monitor.RunTemperatureMonitor(ctx)
	go monitor.RunWiFiMonitor(ctx)

	// Create state machine
	machine := state.New()

	// Start web server
	srv := web.NewServer(machine, version, *configPath)
	if staticFS := web.EmbeddedStaticFS(); staticFS != nil {
		srv.SetStaticFS(staticFS)
	}
	go srv.Start(*listenAddr)

	// Run state machine (blocks until context cancelled)
	if err := machine.Run(ctx); err != nil {
		log.Fatalf("state machine: %v", err)
	}
}
