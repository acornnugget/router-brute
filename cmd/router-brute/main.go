package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nimda/router-brute/internal/core"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v7"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v7/rest"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	debugMode bool
	traceMode bool
)

var rootCmd = &cobra.Command{
	Use:   "router-brute",
	Short: "Router Brute-forcing Tool",
	Long:  "This tool tests password strength on various router platforms.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
		if traceMode {
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
			zlog.Trace().Msg("🔍🔍 TRACE MODE ENABLED")
		} else if debugMode {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
			zlog.Debug().Msg("🔍 DEBUG MODE ENABLED")
		}
	},
}

var mikrotikV6Cmd = &cobra.Command{
	Use:   "mikrotik-v6",
	Short: "Brute force MikroTik RouterOS v6",
	Run:   runMikrotikV6,
}

var mikrotikV7Cmd = &cobra.Command{
	Use:   "mikrotik-v7",
	Short: "Brute force MikroTik RouterOS v7 (binary API)",
	Run:   runMikrotikV7,
}

var mikrotikV7RestCmd = &cobra.Command{
	Use:   "mikrotik-v7-rest",
	Short: "Brute force MikroTik RouterOS v7 (REST API)",
	Run:   runMikrotikV7Rest,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&traceMode, "trace", false, "Enable trace logging")

	// mikrotik-v6 flags
	mikrotikV6Cmd.Flags().String("target", "", "Router IP address or hostname")
	mikrotikV6Cmd.Flags().String("user", "admin", "Username to test")
	mikrotikV6Cmd.Flags().String("wordlist", "", "Path to password wordlist file")
	mikrotikV6Cmd.Flags().Int("workers", 5, "Number of concurrent workers")
	mikrotikV6Cmd.Flags().String("rate", "100ms", "Rate limit between attempts")
	mikrotikV6Cmd.Flags().Int("port", 8728, "Router API port")
	mikrotikV6Cmd.Flags().String("timeout", "10s", "Connection timeout")
	mikrotikV6Cmd.Flags().String("target-file", "", "File containing target specifications (multi-target mode)")
	mikrotikV6Cmd.Flags().Int("concurrent-targets", 1, "Number of targets to attack simultaneously")

	if err := mikrotikV6Cmd.MarkFlagRequired("target"); err != nil {
		log.Fatalf("Failed to mark target flag as required: %v", err)
	}
	if err := mikrotikV6Cmd.MarkFlagRequired("wordlist"); err != nil {
		log.Fatalf("Failed to mark wordlist flag as required: %v", err)
	}

	// mikrotik-v7 flags
	mikrotikV7Cmd.Flags().String("target", "", "Router IP address or hostname")
	mikrotikV7Cmd.Flags().String("user", "admin", "Username to test")
	mikrotikV7Cmd.Flags().String("wordlist", "", "Path to password wordlist file")
	mikrotikV7Cmd.Flags().Int("workers", 5, "Number of concurrent workers")
	mikrotikV7Cmd.Flags().String("rate", "100ms", "Rate limit between attempts")
	mikrotikV7Cmd.Flags().Int("port", 8729, "Router API port (default: 8729)")
	mikrotikV7Cmd.Flags().String("timeout", "10s", "Connection timeout")
	mikrotikV7Cmd.Flags().String("target-file", "", "File containing target specifications (multi-target mode)")
	mikrotikV7Cmd.Flags().Int("concurrent-targets", 1, "Number of targets to attack simultaneously")

	if err := mikrotikV7Cmd.MarkFlagRequired("target"); err != nil {
		log.Fatalf("Failed to mark target flag as required: %v", err)
	}
	if err := mikrotikV7Cmd.MarkFlagRequired("wordlist"); err != nil {
		log.Fatalf("Failed to mark wordlist flag as required: %v", err)
	}

	// mikrotik-v7-rest flags
	mikrotikV7RestCmd.Flags().String("target", "", "Router IP address or hostname")
	mikrotikV7RestCmd.Flags().String("user", "admin", "Username to test")
	mikrotikV7RestCmd.Flags().String("wordlist", "", "Path to password wordlist file")
	mikrotikV7RestCmd.Flags().Int("workers", 5, "Number of concurrent workers")
	mikrotikV7RestCmd.Flags().String("rate", "100ms", "Rate limit between attempts")
	mikrotikV7RestCmd.Flags().Int("port", 80, "HTTP port (default: 80)")
	mikrotikV7RestCmd.Flags().Bool("https", false, "Use HTTPS instead of HTTP")
	mikrotikV7RestCmd.Flags().String("timeout", "10s", "Connection timeout")
	mikrotikV7RestCmd.Flags().String("target-file", "", "File containing target specifications (multi-target mode)")
	mikrotikV7RestCmd.Flags().Int("concurrent-targets", 1, "Number of targets to attack simultaneously")

	if err := mikrotikV7RestCmd.MarkFlagRequired("target"); err != nil {
		log.Fatalf("Failed to mark target flag as required: %v", err)
	}
	if err := mikrotikV7RestCmd.MarkFlagRequired("wordlist"); err != nil {
		log.Fatalf("Failed to mark wordlist flag as required: %v", err)
	}

	rootCmd.AddCommand(mikrotikV6Cmd)
	rootCmd.AddCommand(mikrotikV7Cmd)
	rootCmd.AddCommand(mikrotikV7RestCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runMikrotikV6(cmd *cobra.Command, args []string) {
	target, _ := cmd.Flags().GetString("target")
	user, _ := cmd.Flags().GetString("user")
	wordlist, _ := cmd.Flags().GetString("wordlist")
	workers, _ := cmd.Flags().GetInt("workers")
	rateLimit, _ := cmd.Flags().GetString("rate")
	port, _ := cmd.Flags().GetInt("port")
	timeout, _ := cmd.Flags().GetString("timeout")
	targetFile, _ := cmd.Flags().GetString("target-file")
	concurrentTargets, _ := cmd.Flags().GetInt("concurrent-targets")

	zlog.Debug().Msg("Starting runMikrotikV6 function")
	zlog.Debug().
		Str("target", target).
		Str("user", user).
		Str("wordlist", wordlist).
		Int("workers", workers).
		Str("rate", rateLimit).
		Int("port", port).
		Str("timeout", timeout).
		Str("target-file", targetFile).
		Int("concurrent-targets", concurrentTargets).
		Msg("Flags")

	rateDuration, err := time.ParseDuration(rateLimit)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid rate limit")
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid timeout")
	}

	zlog.Debug().Str("wordlist", wordlist).Msg("Loading passwords from")
	passwords, err := loadPasswords(wordlist)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load wordlist")
	}
	zlog.Debug().Int("n", len(passwords)).Msg("Loaded n passwords")

	// Validate that either target or target-file is specified, but not both
	if targetFile != "" && target != "" {
		zlog.Fatal().Msg("Cannot specify both --target and --target-file")
	}

	if targetFile != "" {
		// Multi-target mode
		zlog.Info().Str("file", targetFile).Msg("Running in multi-target mode")
		runMultiTargetV6(targetFile, wordlist, user, port, timeoutDuration, workers, rateDuration, concurrentTargets)
		return
	}

	// Single-target mode
	if target == "" {
		zlog.Fatal().Msg("Must specify either --target or --target-file")
	}

	zlog.Info().
		Str("target", target).
		Int("passwords", len(passwords)).
		Int("workers", workers).
		Str("rate", rateLimit).
		Msg("Starting attack")

	zlog.Debug().Msg("Creating Mikrotik v6 module")
	module := v6.NewMikrotikV6Module()
	if err := module.Initialize(target, user, map[string]interface{}{
		"port":    port,
		"timeout": timeoutDuration,
	}); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to initialize Mikrotik v6 module")
	}

	zlog.Debug().Int("workers", workers).Dur("ratelimit", rateDuration).Msg("Creating engine")
	engine := core.NewEngine(workers, rateDuration)
	engine.SetModule(module)
	engine.LoadPasswords(passwords)

	zlog.Debug().Msg("Starting engine...")
	if err := engine.Start(); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to start engine")
	}
	zlog.Debug().Msg("Engine started successfully")

	zlog.Debug().Msg("Waiting for results...")
	successCount := 0
	totalAttempts := 0

	for result := range engine.Results() {
		totalAttempts++
		zlog.Trace().
			Int("attempt", totalAttempts).
			Str("password", result.Password).
			Dur("elapsed", result.TimeConsumed).
			Msg("Received result")

		if result.Success {
			successCount++
			zlog.Info().
				Str("username", result.Username).
				Str("password", result.Password).
				Str("target", result.Target).
				Str("module", result.ModuleName).
				Msg("✓ SUCCESS")

			zlog.Debug().Msg("Found valid credentials, stopping engine...")
			engine.Stop()
			zlog.Debug().Msg("Engine stopped")
			break
		}

		if totalAttempts%10 == 0 {
			progress := engine.Progress() * 100
			fmt.Printf("Progress: %.1f%% (%d/%d attempts)\r", progress, totalAttempts, len(passwords))
		}
	}

	zlog.Debug().
		Int("total_attempts", totalAttempts).
		Int("successes", successCount).
		Msg("Results loop completed")

	zlog.Info().Msg("Attack completed")
	zlog.Info().Int("total_attempts", totalAttempts).Msg("Total attempts")
	zlog.Info().Int("successful_attempts", successCount).Msg("Successful authentications")

	if successCount == 0 {
		zlog.Info().Msg("No valid credentials found")
	}
	zlog.Debug().Msg("runMikrotikV6 function completed")
}

func runMikrotikV7(cmd *cobra.Command, args []string) {
	target, _ := cmd.Flags().GetString("target")
	user, _ := cmd.Flags().GetString("user")
	wordlist, _ := cmd.Flags().GetString("wordlist")
	workers, _ := cmd.Flags().GetInt("workers")
	rateLimit, _ := cmd.Flags().GetString("rate")
	port, _ := cmd.Flags().GetInt("port")
	timeout, _ := cmd.Flags().GetString("timeout")
	targetFile, _ := cmd.Flags().GetString("target-file")
	concurrentTargets, _ := cmd.Flags().GetInt("concurrent-targets")

	zlog.Debug().Msg("Starting runMikrotikV7 function")
	zlog.Debug().
		Str("target", target).
		Str("user", user).
		Str("wordlist", wordlist).
		Int("workers", workers).
		Str("rate", rateLimit).
		Int("port", port).
		Str("timeout", timeout).
		Str("target-file", targetFile).
		Int("concurrent-targets", concurrentTargets).
		Msg("Flags")

	rateDuration, err := time.ParseDuration(rateLimit)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid rate limit")
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid timeout")
	}

	zlog.Debug().Str("wordlist", wordlist).Msg("Loading passwords from")
	passwords, err := loadPasswords(wordlist)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load wordlist")
	}
	zlog.Debug().Int("n", len(passwords)).Msg("Loaded n passwords")

	// Validate that either target or target-file is specified, but not both
	if targetFile != "" && target != "" {
		zlog.Fatal().Msg("Cannot specify both --target and --target-file")
	}

	if targetFile != "" {
		// Multi-target mode
		zlog.Info().Str("file", targetFile).Msg("Running in multi-target mode")
		runMultiTargetV7(targetFile, wordlist, user, port, timeoutDuration, workers, rateDuration, concurrentTargets)
		return
	}

	// Single-target mode
	if target == "" {
		zlog.Fatal().Msg("Must specify either --target or --target-file")
	}

	zlog.Info().
		Str("target", target).
		Int("passwords", len(passwords)).
		Int("workers", workers).
		Str("rate", rateLimit).
		Msg("Starting RouterOS v7 attack")

	zlog.Debug().Msg("Creating Mikrotik v7 module")
	module := v7.NewMikrotikV7Module()
	if err := module.Initialize(target, user, map[string]interface{}{
		"port":    port,
		"timeout": timeoutDuration,
	}); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to initialize Mikrotik v7 module")
	}

	zlog.Debug().Int("workers", workers).Dur("ratelimit", rateDuration).Msg("Creating engine")
	engine := core.NewEngine(workers, rateDuration)
	engine.SetModule(module)
	engine.LoadPasswords(passwords)

	zlog.Debug().Msg("Starting engine...")
	if err := engine.Start(); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to start engine")
	}
	zlog.Debug().Msg("Engine started successfully")

	zlog.Debug().Msg("Waiting for results...")
	successCount := 0
	totalAttempts := 0

	for result := range engine.Results() {
		totalAttempts++
		zlog.Trace().
			Int("attempt", totalAttempts).
			Str("password", result.Password).
			Dur("elapsed", result.TimeConsumed).
			Msg("Received result")

		if result.Success {
			successCount++
			zlog.Info().
				Str("username", result.Username).
				Str("password", result.Password).
				Str("target", result.Target).
				Str("module", result.ModuleName).
				Msg("✓ SUCCESS")

			zlog.Debug().Msg("Found valid credentials, stopping engine...")
			engine.Stop()
			zlog.Debug().Msg("Engine stopped")
			break
		}

		if totalAttempts%10 == 0 {
			progress := engine.Progress() * 100
			fmt.Printf("Progress: %.1f%% (%d/%d attempts)\r", progress, totalAttempts, len(passwords))
		}
	}

	zlog.Debug().
		Int("total_attempts", totalAttempts).
		Int("successes", successCount).
		Msg("Results loop completed")

	zlog.Info().Msg("RouterOS v7 attack completed")
	zlog.Info().Int("total_attempts", totalAttempts).Msg("Total attempts")
	zlog.Info().Int("successful_attempts", successCount).Msg("Successful authentications")

	if successCount == 0 {
		zlog.Info().Msg("No valid credentials found")
	}
	zlog.Debug().Msg("runMikrotikV7 function completed")
}

func runMikrotikV7Rest(cmd *cobra.Command, args []string) {
	target, _ := cmd.Flags().GetString("target")
	user, _ := cmd.Flags().GetString("user")
	wordlist, _ := cmd.Flags().GetString("wordlist")
	workers, _ := cmd.Flags().GetInt("workers")
	rateLimit, _ := cmd.Flags().GetString("rate")
	port, _ := cmd.Flags().GetInt("port")
	useHTTPS, _ := cmd.Flags().GetBool("https")
	timeout, _ := cmd.Flags().GetString("timeout")
	targetFile, _ := cmd.Flags().GetString("target-file")
	concurrentTargets, _ := cmd.Flags().GetInt("concurrent-targets")

	zlog.Debug().Msg("Starting runMikrotikV7Rest function")
	zlog.Debug().
		Str("target", target).
		Str("user", user).
		Str("wordlist", wordlist).
		Int("workers", workers).
		Str("rate", rateLimit).
		Int("port", port).
		Bool("https", useHTTPS).
		Str("timeout", timeout).
		Str("target-file", targetFile).
		Int("concurrent-targets", concurrentTargets).
		Msg("Flags")

	rateDuration, err := time.ParseDuration(rateLimit)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid rate limit")
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid timeout")
	}

	zlog.Debug().Str("wordlist", wordlist).Msg("Loading passwords from")
	passwords, err := loadPasswords(wordlist)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load wordlist")
	}
	zlog.Debug().Int("n", len(passwords)).Msg("Loaded n passwords")

	// Validate that either target or target-file is specified, but not both
	if targetFile != "" && target != "" {
		zlog.Fatal().Msg("Cannot specify both --target and --target-file")
	}

	if targetFile != "" {
		// Multi-target mode
		zlog.Info().Str("file", targetFile).Msg("Running in multi-target mode")
		runMultiTargetV7Rest(targetFile, wordlist, user, port, timeoutDuration, workers, rateDuration, concurrentTargets, useHTTPS)
		return
	}

	// Single-target mode
	if target == "" {
		zlog.Fatal().Msg("Must specify either --target or --target-file")
	}

	zlog.Info().
		Str("target", target).
		Int("passwords", len(passwords)).
		Int("workers", workers).
		Str("rate", rateLimit).
		Msg("Starting RouterOS v7 REST API attack")

	zlog.Debug().Msg("Creating Mikrotik v7 REST module")
	module := rest.NewMikrotikV7RestModule()
	if err := module.Initialize(target, user, map[string]interface{}{
		"port":    port,
		"https":   useHTTPS,
		"timeout": timeoutDuration,
	}); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to initialize Mikrotik v7 REST module")
	}

	zlog.Debug().Int("workers", workers).Dur("ratelimit", rateDuration).Msg("Creating engine")
	engine := core.NewEngine(workers, rateDuration)
	engine.SetModule(module)
	engine.LoadPasswords(passwords)

	zlog.Debug().Msg("Starting engine...")
	if err := engine.Start(); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to start engine")
	}
	zlog.Debug().Msg("Engine started successfully")

	zlog.Debug().Msg("Waiting for results...")
	successCount := 0
	totalAttempts := 0

	for result := range engine.Results() {
		totalAttempts++
		zlog.Trace().
			Int("attempt", totalAttempts).
			Str("password", result.Password).
			Dur("elapsed", result.TimeConsumed).
			Msg("Received result")

		if result.Success {
			successCount++
			zlog.Info().
				Str("username", result.Username).
				Str("password", result.Password).
				Str("target", result.Target).
				Str("module", result.ModuleName).
				Msg("✓ SUCCESS")

			zlog.Debug().Msg("Found valid credentials, stopping engine...")
			engine.Stop()
			zlog.Debug().Msg("Engine stopped")
			break
		}

		if totalAttempts%10 == 0 {
			progress := engine.Progress() * 100
			fmt.Printf("Progress: %.1f%% (%d/%d attempts)\r", progress, totalAttempts, len(passwords))
		}
	}

	zlog.Debug().
		Int("total_attempts", totalAttempts).
		Int("successes", successCount).
		Msg("Results loop completed")

	zlog.Info().Msg("RouterOS v7 REST API attack completed")
	zlog.Info().Int("total_attempts", totalAttempts).Msg("Total attempts")
	zlog.Info().Int("successful_attempts", successCount).Msg("Successful authentications")

	if successCount == 0 {
		zlog.Info().Msg("No valid credentials found")
	}
	zlog.Debug().Msg("runMikrotikV7Rest function completed")
}

func runMultiTargetV6(targetFile, wordlist, user string, port int, timeout time.Duration,
	workers int, rateLimit time.Duration, concurrentTargets int) {

	zlog.Info().Str("file", targetFile).Msg("Loading targets for multi-target attack")

	// Load targets
	parser := core.NewTargetParser("", port) // Empty default command
	targets, err := parser.ParseTargetFile(targetFile)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load targets")
	}

	if len(targets) == 0 {
		zlog.Fatal().Msg("No valid targets found in file")
	}

	// Load passwords
	passwords, err := loadPasswords(wordlist)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load wordlist")
	}

	// Create multi-target engine
	factory := &v6.MikrotikV6Factory{}
	engine := core.NewMultiTargetEngine(factory, workers, concurrentTargets, rateLimit)
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Start attack
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.Start(ctx)

	// Process results
	successCount := 0
	for result := range engine.GetResults() {
		if result.Success {
			successCount++
			zlog.Info().
				Str("target", result.Target.IP).
				Str("username", result.Target.Username).
				Str("password", result.SuccessPassword).
				Msg("✓ Found valid credentials")
		}
	}

	// Process errors
	errorCount := 0
	for err := range engine.GetErrors() {
		zlog.Error().
			Str("target", err.Target.IP).
			Err(err.Error).
			Msg("Target processing failed")
		errorCount++
	}

	zlog.Info().
		Int("total_targets", len(targets)).
		Int("successful", successCount).
		Int("failed", errorCount).
		Msg("Multi-target attack summary")
}

func runMultiTargetV7(targetFile, wordlist, user string, port int, timeout time.Duration,
	workers int, rateLimit time.Duration, concurrentTargets int) {

	zlog.Info().Str("file", targetFile).Msg("Loading targets for multi-target attack")

	// Load targets
	parser := core.NewTargetParser("", port) // Empty default command
	targets, err := parser.ParseTargetFile(targetFile)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load targets")
	}

	if len(targets) == 0 {
		zlog.Fatal().Msg("No valid targets found in file")
	}

	// Load passwords
	passwords, err := loadPasswords(wordlist)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load wordlist")
	}

	// Create multi-target engine
	factory := &v7.MikrotikV7Factory{}
	engine := core.NewMultiTargetEngine(factory, workers, concurrentTargets, rateLimit)
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Start attack
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.Start(ctx)

	// Process results
	successCount := 0
	for result := range engine.GetResults() {
		if result.Success {
			successCount++
			zlog.Info().
				Str("target", result.Target.IP).
				Str("username", result.Target.Username).
				Str("password", result.SuccessPassword).
				Msg("✓ Found valid credentials")
		}
	}

	// Process errors
	errorCount := 0
	for err := range engine.GetErrors() {
		zlog.Error().
			Str("target", err.Target.IP).
			Err(err.Error).
			Msg("Target processing failed")
		errorCount++
	}

	zlog.Info().
		Int("total_targets", len(targets)).
		Int("successful", successCount).
		Int("failed", errorCount).
		Msg("Multi-target attack summary")
}

func runMultiTargetV7Rest(targetFile, wordlist, user string, port int, timeout time.Duration,
	workers int, rateLimit time.Duration, concurrentTargets int, useHTTPS bool) {

	zlog.Info().Str("file", targetFile).Msg("Loading targets for multi-target attack")

	// Load targets
	parser := core.NewTargetParser("", port) // Empty default command
	targets, err := parser.ParseTargetFile(targetFile)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load targets")
	}

	if len(targets) == 0 {
		zlog.Fatal().Msg("No valid targets found in file")
	}

	// Load passwords
	passwords, err := loadPasswords(wordlist)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load wordlist")
	}

	// Create multi-target engine
	factory := &rest.MikrotikV7RestFactory{}
	engine := core.NewMultiTargetEngine(factory, workers, concurrentTargets, rateLimit)
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Start attack
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.Start(ctx)

	// Process results
	successCount := 0
	for result := range engine.GetResults() {
		if result.Success {
			successCount++
			zlog.Info().
				Str("target", result.Target.IP).
				Str("username", result.Target.Username).
				Str("password", result.SuccessPassword).
				Msg("✓ Found valid credentials")
		}
	}

	// Process errors
	errorCount := 0
	for err := range engine.GetErrors() {
		zlog.Error().
			Str("target", err.Target.IP).
			Err(err.Error).
			Msg("Target processing failed")
		errorCount++
	}

	zlog.Info().
		Int("total_targets", len(targets)).
		Int("successful", successCount).
		Int("failed", errorCount).
		Msg("Multi-target attack summary")
}

func loadPasswords(filename string) ([]string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var passwords []string
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			passwords = append(passwords, line)
		}
	}
	return passwords, nil
}
