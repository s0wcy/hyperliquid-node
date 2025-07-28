package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"hyperliquid-ws-proxy/config"
	"hyperliquid-ws-proxy/proxy"
	"hyperliquid-ws-proxy/server"
)

const (
	appName    = "Hyperliquid WebSocket Proxy"
	appVersion = "1.0.0"
)

func main() {
	// Parse command line flags
	var (
		configPath = flag.String("config", "", "Path to configuration file")
		logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		logFormat  = flag.String("log-format", "text", "Log format (text, json)")
		version    = flag.Bool("version", false, "Show version information")
		help       = flag.Bool("help", false, "Show help information")
	)
	flag.Parse()

	// Show version
	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		fmt.Println("A WebSocket proxy for Hyperliquid API without rate limits")
		os.Exit(0)
	}

	// Show help
	if *help {
		showHelp()
		os.Exit(0)
	}

	// Setup logging
	setupLogging(*logLevel, *logFormat)

	logrus.WithFields(logrus.Fields{
		"app":     appName,
		"version": appVersion,
	}).Info("Starting application")

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to load configuration")
	}

	logrus.WithFields(logrus.Fields{
		"network":      cfg.Hyperliquid.Network,
		"server_addr":  cfg.GetServerAddress(),
		"max_clients":  cfg.Proxy.MaxClients,
		"local_node":   cfg.Proxy.EnableLocalNode,
	}).Info("Configuration loaded")

	// Create proxy
	p := proxy.NewProxy(cfg)

	// Create server
	srv := server.NewServer(cfg, p)

	// Start proxy
	if err := p.Start(); err != nil {
		logrus.WithError(err).Fatal("Failed to start proxy")
	}

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			logrus.WithError(err).Fatal("Server failed")
		}
	}()

	logrus.WithField("address", cfg.GetServerAddress()).Info("Server started successfully")
	logrus.Info("WebSocket endpoint: ws://" + cfg.GetServerAddress() + "/ws")
	logrus.Info("Health endpoint: http://" + cfg.GetServerAddress() + "/health")
	logrus.Info("Stats endpoint: http://" + cfg.GetServerAddress() + "/stats")

	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	logrus.Info("Received shutdown signal")

	// Graceful shutdown
	logrus.Info("Shutting down server...")
	if err := srv.Stop(); err != nil {
		logrus.WithError(err).Error("Error stopping server")
	}

	logrus.Info("Shutting down proxy...")
	p.Stop()

	logrus.Info("Shutdown complete")
}

// setupLogging configures the logging system
func setupLogging(level, format string) {
	// Set log level
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		logrus.WithError(err).Warn("Invalid log level, using info")
		lvl = logrus.InfoLevel
	}
	logrus.SetLevel(lvl)

	// Set log format
	switch format {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	case "text":
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	default:
		logrus.WithField("format", format).Warn("Invalid log format, using text")
		logrus.SetFormatter(&logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	}

	// Log to stdout
	logrus.SetOutput(os.Stdout)
}

// showHelp displays help information
func showHelp() {
	fmt.Printf("%s v%s\n\n", appName, appVersion)
	fmt.Println("A WebSocket proxy for Hyperliquid API without rate limits")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  hyperliquid-ws-proxy [OPTIONS]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  -config string")
	fmt.Println("        Path to configuration file")
	fmt.Println("  -log-level string")
	fmt.Println("        Log level (debug, info, warn, error) (default \"info\")")
	fmt.Println("  -log-format string")
	fmt.Println("        Log format (text, json) (default \"text\")")
	fmt.Println("  -version")
	fmt.Println("        Show version information")
	fmt.Println("  -help")
	fmt.Println("        Show this help message")
	fmt.Println()
	fmt.Println("ENDPOINTS:")
	fmt.Println("  WebSocket: ws://localhost:8080/ws")
	fmt.Println("  Health:    http://localhost:8080/health")
	fmt.Println("  Stats:     http://localhost:8080/stats")
	fmt.Println("  Info:      http://localhost:8080/info")
	fmt.Println()
	fmt.Println("EXAMPLE USAGE:")
	fmt.Println("  # Start with default configuration")
	fmt.Println("  ./hyperliquid-ws-proxy")
	fmt.Println()
	fmt.Println("  # Start with custom configuration")
	fmt.Println("  ./hyperliquid-ws-proxy -config config.yaml")
	fmt.Println()
	fmt.Println("  # Start with debug logging")
	fmt.Println("  ./hyperliquid-ws-proxy -log-level debug")
	fmt.Println()
	fmt.Println("SUPPORTED SUBSCRIPTIONS:")
	fmt.Println("  - allMids: All mid prices")
	fmt.Println("  - l2Book: Order book snapshots")
	fmt.Println("  - trades: Trade updates")
	fmt.Println("  - candle: Candlestick data")
	fmt.Println("  - bbo: Best bid/offer")
	fmt.Println("  - notification: User notifications")
	fmt.Println("  - webData2: Web interface data")
	fmt.Println("  - orderUpdates: Order status updates")
	fmt.Println("  - userEvents: User events")
	fmt.Println("  - userFills: User fill history")
	fmt.Println("  - userFundings: User funding payments")
	fmt.Println("  - userNonFundingLedgerUpdates: Ledger updates")
	fmt.Println("  - activeAssetCtx: Asset context")
	fmt.Println("  - activeAssetData: Active asset data")
	fmt.Println("  - userTwapSliceFills: TWAP slice fills")
	fmt.Println("  - userTwapHistory: TWAP history")
	fmt.Println()
	fmt.Println("FEATURES:")
	fmt.Println("  ✓ No rate limits")
	fmt.Println("  ✓ Multiple client support")
	fmt.Println("  ✓ Automatic reconnection")
	fmt.Println("  ✓ Local node integration")
	fmt.Println("  ✓ POST request support")
	fmt.Println("  ✓ Real-time statistics")
	fmt.Println("  ✓ Health monitoring")
	fmt.Println()
} 