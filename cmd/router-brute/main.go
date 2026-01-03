package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nimda/router-brute/internal/core"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	loggingMode string
	debugMode   bool
	traceMode   bool
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

	mikrotikV6Cmd.MarkFlagRequired("target")
	mikrotikV6Cmd.MarkFlagRequired("wordlist")

	rootCmd.AddCommand(mikrotikV6Cmd)
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

	zlog.Debug().Msg("Starting runMikrotikV6 function")
	zlog.Debug().
		Str("target", target).
		Str("user", user).
		Str("wordlist", wordlist).
		Int("workers", workers).
		Str("rate", rateLimit).
		Int("port", port).
		Str("timeout", timeout).
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

	zlog.Info().
		Str("target", target).
		Int("passwords", len(passwords)).
		Int("workers", workers).
		Str("rate", rateLimit).
		Msg("Starting attack")

	zlog.Debug().Msg("Creating Mikrotik v6 module")
	module := v6.NewMikrotikV6Module(loggingMode)
	module.Initialize(target, user, map[string]interface{}{
		"port":    port,
		"timeout": timeoutDuration,
	})

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
