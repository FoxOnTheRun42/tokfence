package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/macfox/tokfence/internal/budget"
	"github.com/macfox/tokfence/internal/config"
	"github.com/macfox/tokfence/internal/daemon"
	"github.com/macfox/tokfence/internal/logger"
	"github.com/macfox/tokfence/internal/vault"
)

var (
	configPath string
	outputJSON bool
)

const daemonNonceEnv = "TOKFENCE_DAEMON_NONCE"

func main() {
	rootCmd := &cobra.Command{
		Use:   "tokfence",
		Short: "Tokfence is a local-first AI API gateway and key vault",
	}
	rootCmd.PersistentFlags().StringVar(&configPath, "config", config.DefaultConfigPath(), "path to config file")
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "output JSON")

	rootCmd.AddCommand(newStartCommand())
	rootCmd.AddCommand(newStopCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newVaultCommand())
	rootCmd.AddCommand(newLogCommand())
	rootCmd.AddCommand(newStatsCommand())
	rootCmd.AddCommand(newBudgetCommand())
	rootCmd.AddCommand(newRateLimitCommand())
	rootCmd.AddCommand(newRevokeCommand())
	rootCmd.AddCommand(newRestoreCommand())
	rootCmd.AddCommand(newKillCommand())
	rootCmd.AddCommand(newUnkillCommand())
	rootCmd.AddCommand(newEnvCommand())
	rootCmd.AddCommand(newWidgetCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func newStartCommand() *cobra.Command {
	var daemonize bool
	var daemonNonce string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start tokfence daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if daemonNonce == "" {
				daemonNonce = os.Getenv(daemonNonceEnv)
			}
			if daemonize && os.Getenv("TOKFENCE_BACKGROUND") != "1" {
				return spawnBackground(cfg, daemonNonce)
			}
			return runForeground(cfg, daemonNonce)
		},
	}
	cmd.Flags().BoolVarP(&daemonize, "daemon", "d", false, "run in background")
	cmd.Flags().StringVar(&daemonNonce, "tokfence-daemon-nonce", "", "internal daemon nonce")
	if flag := cmd.Flags().Lookup("tokfence-daemon-nonce"); flag != nil {
		flag.Hidden = true
	}
	return cmd
}

func runForeground(cfg config.Config, daemonNonce string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if _, err := config.EnsureSecureDataDir(); err != nil {
		return err
	}
	store, err := logger.Open(cfg.Logging.DBPath)
	if err != nil {
		return err
	}
	defer store.Close()

	v, err := vault.NewDefault(vault.Options{})
	if err != nil {
		return fmt.Errorf("init vault: %w", err)
	}

	engine := budget.NewEngine(store.DB())
	server := daemon.NewServer(cfg, v, store, engine)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve daemon binary path: %w", err)
	}
	resolvedExecPath := execPath
	if expanded, expandErr := config.ExpandPath(execPath); expandErr == nil && expanded != "" {
		resolvedExecPath = expanded
	}
	execPath = resolvedExecPath
	nonce := strings.TrimSpace(daemonNonce)
	if nonce == "" {
		var err error
		nonce, err = randomHex(16)
		if err != nil {
			return fmt.Errorf("generate daemon nonce: %w", err)
		}
	}
	if err := writePIDFile(os.Getpid(), server.Addr(), execPath, os.Getuid(), nonce); err != nil {
		return err
	}
	defer removePIDFile()

	if !outputJSON {
		fmt.Printf("tokfence daemon listening on http://%s\n", server.Addr())
	}
	err = server.Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func spawnBackground(cfg config.Config, daemonNonce string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	nonce := strings.TrimSpace(daemonNonce)
	if nonce == "" {
		nonce, err = randomHex(16)
		if err != nil {
			return fmt.Errorf("generate daemon nonce: %w", err)
		}
	}
	args := []string{"start", "--config", configPath, "--tokfence-daemon-nonce", nonce}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "TOKFENCE_BACKGROUND=1", daemonNonceEnv+"="+nonce)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	logPath := filepath.Join(mustDataDir(), "tokfence.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open daemon log file: %w", err)
	}
	defer logFile.Close()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start background daemon: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("detach daemon process: %w", err)
	}
	if outputJSON {
		return printJSON(map[string]any{"status": "starting", "addr": fmt.Sprintf("%s:%d", cfg.Daemon.Host, cfg.Daemon.Port)})
	}
	fmt.Printf("tokfence daemon starting in background (http://%s:%d)\n", cfg.Daemon.Host, cfg.Daemon.Port)
	return nil
}

func newStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop tokfence daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			state, err := readPIDFile()
			if err != nil {
				return err
			}
			if err := verifyDaemonProcess(state); err != nil {
				return err
			}
			if err := syscall.Kill(state.PID, syscall.SIGTERM); err != nil {
				if errors.Is(err, os.ErrProcessDone) {
					removePIDFile()
					return nil
				}
				return fmt.Errorf("stop daemon: %w", err)
			}
			deadline := time.Now().Add(30 * time.Second)
			for time.Now().Before(deadline) {
				alive := isProtectedProcessAlive(state)
				if !alive {
					removePIDFile()
					if outputJSON {
						return printJSON(map[string]any{"status": "stopped"})
					}
					fmt.Println("tokfence daemon stopped")
					return nil
				}
				time.Sleep(150 * time.Millisecond)
			}
			return errors.New("timed out while waiting for daemon to stop")
		},
	}
}

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			state, err := readPIDFile()
			if err != nil {
				if outputJSON {
					return printJSON(map[string]any{"running": false})
				}
				fmt.Println("tokfence daemon is not running")
				return nil
			}
			running, reason := protectedProcessState(state)
			if !running && reason != "" {
				if outputJSON {
					return printJSON(map[string]any{
						"running": false,
						"pid":     state.PID,
						"addr":    state.Addr,
						"started": state.StartedAt,
						"error":   reason,
					})
				}
				fmt.Printf("tokfence daemon is not running (%s)\n", reason)
				return nil
			}
			if outputJSON {
				return printJSON(map[string]any{
					"running": running,
					"pid":     state.PID,
					"addr":    state.Addr,
					"started": state.StartedAt,
				})
			}
			if !running {
				fmt.Println("tokfence daemon is not running")
				return nil
			}
			fmt.Printf("tokfence daemon is running\nPID: %d\nAddr: %s\nStarted: %s\n", state.PID, state.Addr, state.StartedAt)
			return nil
		},
	}
}

func newVaultCommand() *cobra.Command {
	vaultCmd := &cobra.Command{Use: "vault", Short: "Manage provider API keys"}

	vaultCmd.AddCommand(&cobra.Command{
		Use:   "add <provider> <key|- >",
		Short: "Add or update API key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			if err := vault.ValidateProvider(provider); err != nil {
				return err
			}
			key := args[1]
			if key == "-" {
				in, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read key from stdin: %w", err)
				}
				key = strings.TrimSpace(string(in))
			}
			v, err := vault.NewDefault(vault.Options{})
			if err != nil {
				return err
			}
			if err := v.Set(context.Background(), provider, key); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "status": "stored"})
			}
			fmt.Printf("stored key for %s\n", provider)
			return nil
		},
	})

	vaultCmd.AddCommand(&cobra.Command{
		Use:   "remove <provider>",
		Short: "Remove API key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			if err := vault.ValidateProvider(provider); err != nil {
				return err
			}
			v, err := vault.NewDefault(vault.Options{})
			if err != nil {
				return err
			}
			if err := v.Delete(context.Background(), provider); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "status": "removed"})
			}
			fmt.Printf("removed key for %s\n", provider)
			return nil
		},
	})

	vaultCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List providers with stored keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, err := vault.NewDefault(vault.Options{})
			if err != nil {
				return err
			}
			providers, err := v.List(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"providers": providers})
			}
			if len(providers) == 0 {
				fmt.Println("no providers configured")
				return nil
			}
			for _, provider := range providers {
				fmt.Println(provider)
			}
			return nil
		},
	})

	vaultCmd.AddCommand(&cobra.Command{
		Use:   "rotate <provider> <new-key|- >",
		Short: "Rotate API key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			if err := vault.ValidateProvider(provider); err != nil {
				return err
			}
			key := args[1]
			if key == "-" {
				in, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read key from stdin: %w", err)
				}
				key = strings.TrimSpace(string(in))
			}
			v, err := vault.NewDefault(vault.Options{})
			if err != nil {
				return err
			}
			if err := v.Set(context.Background(), provider, key); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "status": "rotated"})
			}
			fmt.Printf("rotated key for %s\n", provider)
			return nil
		},
	})

	vaultCmd.AddCommand(&cobra.Command{
		Use:   "export",
		Short: "Export encrypted vault backup",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := config.ExpandPath("~/.tokfence/vault.enc")
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read vault file: %w", err)
			}
			if _, err := os.Stdout.Write(data); err != nil {
				return fmt.Errorf("write export: %w", err)
			}
			return nil
		},
	})

	vaultCmd.AddCommand(&cobra.Command{
		Use:   "import <file>",
		Short: "Import encrypted vault backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			inPath, err := config.ExpandPath(args[0])
			if err != nil {
				return err
			}
			data, err := os.ReadFile(inPath)
			if err != nil {
				return fmt.Errorf("read import file: %w", err)
			}
			outPath, err := config.ExpandPath("~/.tokfence/vault.enc")
			if err != nil {
				return err
			}
			if err := os.WriteFile(outPath, data, 0o600); err != nil {
				return fmt.Errorf("write vault file: %w", err)
			}
			if outputJSON {
				return printJSON(map[string]any{"status": "imported", "file": outPath})
			}
			fmt.Printf("imported vault from %s\n", inPath)
			return nil
		},
	})

	return vaultCmd
}

func newLogCommand() *cobra.Command {
	var provider string
	var since string
	var model string
	var follow bool

	cmd := &cobra.Command{
		Use:   "log [request-id]",
		Short: "Show request logs",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()

			if len(args) == 1 {
				record, err := store.GetRequest(context.Background(), args[0])
				if err != nil {
					return err
				}
				if outputJSON {
					return printJSON(record)
				}
				fmt.Printf("%s %s %s %s %d cost=$%.2f\n", record.ID, record.Timestamp.Format(time.RFC3339), record.Provider, record.Endpoint, record.StatusCode, float64(record.EstimatedCostCents)/100.0)
				fmt.Printf("model=%s in=%d out=%d caller=%s latency=%dms\n", record.Model, record.InputTokens, record.OutputTokens, record.CallerName, record.LatencyMS)
				if record.ErrorType != "" {
					fmt.Printf("error=%s %s\n", record.ErrorType, record.ErrorMessage)
				}
				return nil
			}

			sinceTime, err := parseWindowStart(since)
			if err != nil {
				return err
			}

			if follow {
				return followLogs(store, provider, model, sinceTime)
			}

			records, err := store.ListRequests(context.Background(), logger.QueryFilter{Limit: 20, Provider: provider, Model: model, Since: sinceTime})
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(records)
			}
			for i := len(records) - 1; i >= 0; i-- {
				rec := records[i]
				fmt.Printf("%s %s %s %s %d in=%d out=%d cost=$%.2f latency=%dms\n",
					rec.ID,
					rec.Timestamp.Format(time.RFC3339),
					rec.Provider,
					rec.Model,
					rec.StatusCode,
					rec.InputTokens,
					rec.OutputTokens,
					float64(rec.EstimatedCostCents)/100.0,
					rec.LatencyMS,
				)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "filter by provider")
	cmd.Flags().StringVar(&since, "since", "", "time window (e.g. 1h, 24h)")
	cmd.Flags().StringVar(&model, "model", "", "filter by model")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow logs")
	return cmd
}

func followLogs(store *logger.LogStore, provider, model string, since time.Time) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	latest := since
	encoder := json.NewEncoder(os.Stdout)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
			records, err := store.ListRequests(context.Background(), logger.QueryFilter{Limit: 200, Provider: provider, Model: model, Since: latest})
			if err != nil {
				return err
			}
			sort.Slice(records, func(i, j int) bool {
				return records[i].Timestamp.Before(records[j].Timestamp)
			})
			for _, rec := range records {
				if !rec.Timestamp.After(latest) {
					continue
				}
				if outputJSON {
					if err := encoder.Encode(rec); err != nil {
						return err
					}
				} else {
					fmt.Printf("%s %s %s %s %d in=%d out=%d cost=$%.2f\n",
						rec.ID,
						rec.Timestamp.Format(time.RFC3339),
						rec.Provider,
						rec.Model,
						rec.StatusCode,
						rec.InputTokens,
						rec.OutputTokens,
						float64(rec.EstimatedCostCents)/100.0,
					)
				}
				latest = rec.Timestamp
			}
		}
	}
}

func newStatsCommand() *cobra.Command {
	var period string
	var provider string
	var groupBy string
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show usage and cost statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			since, err := parseStatsPeriod(period)
			if err != nil {
				return err
			}
			rows, err := store.Stats(context.Background(), logger.StatsFilter{Provider: provider, Since: since, By: groupBy})
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(rows)
			}
			if len(rows) == 0 {
				fmt.Println("no usage data")
				return nil
			}
			for _, row := range rows {
				fmt.Printf("%s requests=%d in=%d out=%d cost=$%.2f\n", row.Group, row.RequestCount, row.InputTokens, row.OutputTokens, float64(row.EstimatedCostCents)/100.0)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&period, "period", "today", "time period (today, 7d, 30d, 24h)")
	cmd.Flags().StringVar(&provider, "provider", "", "provider filter")
	cmd.Flags().StringVar(&groupBy, "by", "provider", "group by: provider, model, hour")
	return cmd
}

func newBudgetCommand() *cobra.Command {
	budgetCmd := &cobra.Command{Use: "budget", Short: "Manage spend limits"}

	budgetCmd.AddCommand(&cobra.Command{
		Use:   "set <provider> <amount_usd> <period>",
		Short: "Set a budget limit",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			if provider != "global" {
				if err := vault.ValidateProvider(provider); err != nil {
					return err
				}
			}
			amount, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("parse amount: %w", err)
			}
			period := strings.ToLower(args[2])
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			engine := budget.NewEngine(store.DB())
			if err := engine.SetBudget(context.Background(), provider, amount, period); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "amount": amount, "period": period, "status": "set"})
			}
			fmt.Printf("budget set for %s: $%.2f (%s)\n", provider, amount, period)
			return nil
		},
	})

	budgetCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show budget status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			engine := budget.NewEngine(store.DB())
			rows, err := engine.Status(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(rows)
			}
			if len(rows) == 0 {
				fmt.Println("no budgets configured")
				return nil
			}
			for _, row := range rows {
				fmt.Printf("%s $%.2f / $%.2f (%s) reset=%s\n", row.Provider, float64(row.CurrentSpendCents)/100.0, float64(row.LimitCents)/100.0, row.Period, row.PeriodStart.Format(time.RFC3339))
			}
			return nil
		},
	})

	budgetCmd.AddCommand(&cobra.Command{
		Use:   "clear <provider>",
		Short: "Clear budget limit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			engine := budget.NewEngine(store.DB())
			if err := engine.ClearBudget(context.Background(), provider); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "status": "cleared"})
			}
			fmt.Printf("budget cleared for %s\n", provider)
			return nil
		},
	})

	return budgetCmd
}

func newRateLimitCommand() *cobra.Command {
	rateCmd := &cobra.Command{Use: "ratelimit", Short: "Manage per-provider rate limits"}

	rateCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show configured requests-per-minute limits",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			limits, err := store.ListRateLimits(context.Background())
			if err != nil {
				return err
			}
			if outputJSON {
				return printJSON(limits)
			}
			if len(limits) == 0 {
				fmt.Println("no rate limits configured")
				return nil
			}
			providers := make([]string, 0, len(limits))
			for provider := range limits {
				providers = append(providers, provider)
			}
			sort.Strings(providers)
			for _, provider := range providers {
				fmt.Printf("%s: %d rpm\n", provider, limits[provider])
			}
			return nil
		},
	})

	rateCmd.AddCommand(&cobra.Command{
		Use:   "set <provider> <rpm>",
		Short: "Set requests-per-minute limit",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			if err := vault.ValidateProvider(provider); err != nil {
				return err
			}
			rpm, err := strconv.Atoi(args[1])
			if err != nil || rpm <= 0 {
				return errors.New("rpm must be a positive integer")
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.SetRateLimit(context.Background(), provider, rpm); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "rpm": rpm, "status": "set"})
			}
			fmt.Printf("rate limit set for %s: %d rpm\n", provider, rpm)
			return nil
		},
	})

	rateCmd.AddCommand(&cobra.Command{
		Use:   "clear <provider>",
		Short: "Clear requests-per-minute limit",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.ClearRateLimit(context.Background(), provider); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "status": "cleared"})
			}
			fmt.Printf("rate limit cleared for %s\n", provider)
			return nil
		},
	})

	return rateCmd
}

func newRevokeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "revoke <provider>",
		Short: "Revoke provider access",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			if err := vault.ValidateProvider(provider); err != nil {
				return err
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.SetProviderRevoked(context.Background(), provider, true); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "revoked": true})
			}
			fmt.Printf("provider revoked: %s\n", provider)
			return nil
		},
	}
}

func newRestoreCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <provider>",
		Short: "Restore provider access",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(args[0])
			if err := vault.ValidateProvider(provider); err != nil {
				return err
			}
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.SetProviderRevoked(context.Background(), provider, false); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"provider": provider, "revoked": false})
			}
			fmt.Printf("provider restored: %s\n", provider)
			return nil
		},
	}
}

func newKillCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "kill",
		Short: "Revoke all providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			providers := make([]string, 0, len(cfg.Providers))
			for provider := range cfg.Providers {
				providers = append(providers, provider)
			}
			if err := store.SetAllProvidersRevoked(context.Background(), providers, true); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"revoked": providers})
			}
			fmt.Println("all providers revoked")
			return nil
		},
	}
}

func newUnkillCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unkill",
		Short: "Restore all providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			store, err := logger.Open(cfg.Logging.DBPath)
			if err != nil {
				return err
			}
			defer store.Close()
			providers := make([]string, 0, len(cfg.Providers))
			for provider := range cfg.Providers {
				providers = append(providers, provider)
			}
			if err := store.SetAllProvidersRevoked(context.Background(), providers, false); err != nil {
				return err
			}
			if outputJSON {
				return printJSON(map[string]any{"restored": providers})
			}
			fmt.Println("all providers restored")
			return nil
		},
	}
}

func newEnvCommand() *cobra.Command {
	var shell string
	var provider string
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print shell exports for base URLs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			selected := map[string]config.ProviderConfig{}
			if provider != "" {
				p, ok := cfg.Providers[provider]
				if !ok {
					return fmt.Errorf("unknown provider %s", provider)
				}
				selected[provider] = p
			} else {
				selected = cfg.Providers
			}
			names := make([]string, 0, len(selected))
			for name := range selected {
				names = append(names, name)
			}
			sort.Strings(names)

			if outputJSON {
				entries := map[string]string{}
				for _, name := range names {
					entries[strings.ToUpper(name)+"_BASE_URL"] = fmt.Sprintf("http://%s:%d/%s", cfg.Daemon.Host, cfg.Daemon.Port, name)
				}
				return printJSON(entries)
			}

			for _, name := range names {
				envName := strings.ToUpper(name) + "_BASE_URL"
				value := fmt.Sprintf("http://%s:%d/%s", cfg.Daemon.Host, cfg.Daemon.Port, name)
				switch shell {
				case "fish":
					fmt.Printf("set -x %s %q;\n", envName, value)
				default:
					fmt.Printf("export %s=%q\n", envName, value)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "zsh", "shell format: bash, zsh, fish")
	cmd.Flags().StringVar(&provider, "provider", "", "single provider")
	return cmd
}

func newWidgetCommand() *cobra.Command {
	widgetCmd := &cobra.Command{
		Use:   "widget",
		Short: "Desktop UI integration for SwiftBar",
	}

	var pluginDir string
	var binaryPath string
	var refreshSeconds int
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install the Tokfence SwiftBar plugin",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if refreshSeconds < 5 {
				return errors.New("refresh must be >= 5 seconds")
			}
			if pluginDir == "" {
				pluginDir = "~/Library/Application Support/SwiftBar/Plugins"
			}
			dir, err := config.ExpandPath(pluginDir)
			if err != nil {
				return fmt.Errorf("expand plugin directory: %w", err)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create plugin directory: %w", err)
			}

			if binaryPath == "" {
				binaryPath, err = os.Executable()
				if err != nil {
					return fmt.Errorf("resolve tokfence binary path: %w", err)
				}
			}
			binaryPath, err = filepath.Abs(binaryPath)
			if err != nil {
				return fmt.Errorf("resolve absolute binary path: %w", err)
			}

			expandedConfigPath := configPath
			if strings.TrimSpace(expandedConfigPath) != "" {
				expandedConfigPath, err = config.ExpandPath(expandedConfigPath)
				if err != nil {
					return fmt.Errorf("expand config path: %w", err)
				}
			}

			pluginPath := filepath.Join(dir, fmt.Sprintf("tokfence.%ds.sh", refreshSeconds))
			script := buildSwiftBarPluginScript(binaryPath, expandedConfigPath)
			if err := os.WriteFile(pluginPath, []byte(script), 0o755); err != nil {
				return fmt.Errorf("write SwiftBar plugin: %w", err)
			}
			if err := os.Chmod(pluginPath, 0o755); err != nil {
				return fmt.Errorf("set plugin executable bit: %w", err)
			}

			if outputJSON {
				return printJSON(map[string]any{
					"status":      "installed",
					"plugin_path": pluginPath,
					"refresh_sec": refreshSeconds,
					"binary_path": binaryPath,
				})
			}

			fmt.Printf("SwiftBar plugin installed:\n%s\n\n", pluginPath)
			fmt.Println("Next steps:")
			fmt.Println("1. Open SwiftBar (or click Refresh All in SwiftBar).")
			fmt.Println("2. Add Tokfence to your menu bar from the plugin list.")
			fmt.Println("3. Use the Tokfence menu actions directly from the menubar UI.")
			return nil
		},
	}
	installCmd.Flags().StringVar(&pluginDir, "plugins-dir", "", "SwiftBar plugin directory")
	installCmd.Flags().StringVar(&binaryPath, "binary", "", "path to tokfence binary")
	installCmd.Flags().IntVar(&refreshSeconds, "refresh", 20, "widget refresh interval in seconds")

	var uninstallDir string
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove Tokfence SwiftBar plugin files",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if uninstallDir == "" {
				uninstallDir = "~/Library/Application Support/SwiftBar/Plugins"
			}
			dir, err := config.ExpandPath(uninstallDir)
			if err != nil {
				return fmt.Errorf("expand plugin directory: %w", err)
			}
			patterns := []string{
				filepath.Join(dir, "tokfence.*.sh"),
				filepath.Join(dir, "tokfence.sh"),
			}
			removed := 0
			for _, pattern := range patterns {
				matches, _ := filepath.Glob(pattern)
				for _, match := range matches {
					if err := os.Remove(match); err == nil {
						removed++
					}
				}
			}
			if outputJSON {
				return printJSON(map[string]any{"status": "uninstalled", "removed_files": removed, "plugin_dir": dir})
			}
			fmt.Printf("Removed %d Tokfence SwiftBar plugin file(s) from %s\n", removed, dir)
			return nil
		},
	}
	uninstallCmd.Flags().StringVar(&uninstallDir, "plugins-dir", "", "SwiftBar plugin directory")

	renderCmd := &cobra.Command{
		Use:   "render",
		Short: "Render SwiftBar widget output",
		RunE: func(cmd *cobra.Command, _ []string) error {
			snapshot := collectWidgetSnapshot(context.Background())
			if outputJSON {
				return printJSON(snapshot)
			}
			renderSwiftBarWidget(snapshot)
			return nil
		},
	}

	widgetCmd.AddCommand(installCmd)
	widgetCmd.AddCommand(uninstallCmd)
	widgetCmd.AddCommand(renderCmd)
	return widgetCmd
}

func buildSwiftBarPluginScript(binaryPath, cfgPath string) string {
	commandParts := []string{shellQuote(binaryPath), "widget", "render"}
	if strings.TrimSpace(cfgPath) != "" {
		commandParts = append(commandParts, "--config", shellQuote(cfgPath))
	}
	command := strings.Join(commandParts, " ")
	return "#!/usr/bin/env bash\nset -euo pipefail\n\nif ! " + command + "; then\n  echo \"🛡️ Tokfence • error | color=#ef4444\"\n  echo \"---\"\n  echo \"Widget render failed\"\nfi\n"
}

func shellQuote(raw string) string {
	return "'" + strings.ReplaceAll(raw, "'", `'"'"'`) + "'"
}

type widgetSnapshot struct {
	GeneratedAt       time.Time         `json:"generated_at"`
	Running           bool              `json:"running"`
	PID               int               `json:"pid,omitempty"`
	Addr              string            `json:"addr,omitempty"`
	TodayRequests     int               `json:"today_requests"`
	TodayInputTokens  int64             `json:"today_input_tokens"`
	TodayOutputTokens int64             `json:"today_output_tokens"`
	TodayCostCents    int64             `json:"today_cost_cents"`
	TopProvider       string            `json:"top_provider,omitempty"`
	TopProviderCents  int64             `json:"top_provider_cost_cents,omitempty"`
	Budgets           []budget.Budget   `json:"budgets"`
	RevokedProviders  []string          `json:"revoked_providers"`
	VaultProviders    []string          `json:"vault_providers"`
	Providers         []string          `json:"providers,omitempty"`
	ProviderUpstreams map[string]string `json:"provider_upstreams,omitempty"`
	RateLimits        map[string]int    `json:"rate_limits,omitempty"`
	KillSwitchActive  bool              `json:"kill_switch_active"`
	LastRequestAt     string            `json:"last_request_at,omitempty"`
	Warnings          []string          `json:"warnings,omitempty"`
}

func collectWidgetSnapshot(ctx context.Context) widgetSnapshot {
	snapshot := widgetSnapshot{
		GeneratedAt:       time.Now().UTC(),
		Budgets:           []budget.Budget{},
		RevokedProviders:  []string{},
		VaultProviders:    []string{},
		Providers:         []string{},
		ProviderUpstreams: map[string]string{},
		RateLimits:        map[string]int{},
		Warnings:          []string{},
	}

	if state, err := readPIDFile(); err == nil {
		running, reason := protectedProcessState(state)
		snapshot.Running = running
		if reason != "" {
			snapshot.Warnings = append(snapshot.Warnings, "daemon: "+reason)
		}
		snapshot.PID = state.PID
		snapshot.Addr = state.Addr
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		snapshot.Warnings = append(snapshot.Warnings, "config: "+err.Error())
		return snapshot
	}

	store, err := logger.Open(cfg.Logging.DBPath)
	if err != nil {
		snapshot.Warnings = append(snapshot.Warnings, "db: "+err.Error())
	} else {
		defer store.Close()

		now := time.Now().In(time.Local)
		todayStartLocal := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		rows, statsErr := store.Stats(ctx, logger.StatsFilter{Since: todayStartLocal.UTC(), By: "provider"})
		if statsErr != nil {
			snapshot.Warnings = append(snapshot.Warnings, "stats: "+statsErr.Error())
		} else {
			for _, row := range rows {
				snapshot.TodayRequests += row.RequestCount
				snapshot.TodayInputTokens += row.InputTokens
				snapshot.TodayOutputTokens += row.OutputTokens
				snapshot.TodayCostCents += row.EstimatedCostCents
				if row.EstimatedCostCents > snapshot.TopProviderCents {
					snapshot.TopProvider = row.Group
					snapshot.TopProviderCents = row.EstimatedCostCents
				}
			}
		}

		last, lastErr := store.ListRequests(ctx, logger.QueryFilter{Limit: 1})
		if lastErr != nil {
			snapshot.Warnings = append(snapshot.Warnings, "last-request: "+lastErr.Error())
		} else if len(last) > 0 {
			snapshot.LastRequestAt = last[0].Timestamp.In(time.Local).Format("15:04:05")
		}

		engine := budget.NewEngine(store.DB())
		budgets, budgetErr := engine.Status(ctx)
		if budgetErr != nil {
			snapshot.Warnings = append(snapshot.Warnings, "budgets: "+budgetErr.Error())
		} else {
			snapshot.Budgets = budgets
		}

		providers := make([]string, 0, len(cfg.Providers))
		for provider := range cfg.Providers {
			providers = append(providers, provider)
		}
		sort.Strings(providers)
		snapshot.Providers = append(snapshot.Providers, providers...)
		for _, provider := range providers {
			snapshot.ProviderUpstreams[provider] = cfg.Providers[provider].Upstream
		}
		for _, provider := range providers {
			revoked, revokedErr := store.IsProviderRevoked(ctx, provider)
			if revokedErr != nil {
				snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("provider-status %s: %v", provider, revokedErr))
				continue
			}
			if revoked {
				snapshot.RevokedProviders = append(snapshot.RevokedProviders, provider)
			}
		}
		limits, limitErr := store.ListRateLimits(ctx)
		if limitErr != nil {
			snapshot.Warnings = append(snapshot.Warnings, "ratelimits: "+limitErr.Error())
		} else {
			snapshot.RateLimits = limits
		}
		if len(providers) > 0 && len(snapshot.RevokedProviders) == len(providers) {
			snapshot.KillSwitchActive = true
		}
	}

	v, err := vault.NewDefault(vault.Options{})
	if err != nil {
		snapshot.Warnings = append(snapshot.Warnings, "vault: "+err.Error())
	} else {
		providers, listErr := v.List(ctx)
		if listErr != nil {
			snapshot.Warnings = append(snapshot.Warnings, "vault-list: "+listErr.Error())
		} else {
			snapshot.VaultProviders = providers
		}
	}

	return snapshot
}

func renderSwiftBarWidget(snapshot widgetSnapshot) {
	if snapshot.Running {
		fmt.Printf("🛡️ Tokfence %s · %d req | color=#16a34a\n", formatUSD(snapshot.TodayCostCents), snapshot.TodayRequests)
	} else {
		fmt.Println("🛡️ Tokfence offline | color=#ef4444")
	}
	fmt.Println("---")
	fmt.Println("Tokfence Dashboard | color=#6b7280 size=12")
	if snapshot.Running {
		fmt.Printf("Status: Running ✅ (PID %d)\n", snapshot.PID)
		if snapshot.Addr != "" {
			fmt.Printf("Address: http://%s\n", snapshot.Addr)
		}
	} else {
		fmt.Println("Status: Offline")
	}
	if snapshot.LastRequestAt != "" {
		fmt.Printf("Last Request: %s\n", snapshot.LastRequestAt)
	}
	fmt.Println("---")
	fmt.Println("Today | color=#6b7280")
	fmt.Printf("Requests: %d\n", snapshot.TodayRequests)
	fmt.Printf("Cost: %s\n", formatUSD(snapshot.TodayCostCents))
	fmt.Printf("Input Tokens: %s\n", formatTokenCount(snapshot.TodayInputTokens))
	fmt.Printf("Output Tokens: %s\n", formatTokenCount(snapshot.TodayOutputTokens))
	if snapshot.TopProvider != "" {
		fmt.Printf("Top Provider: %s (%s)\n", snapshot.TopProvider, formatUSD(snapshot.TopProviderCents))
	}

	fmt.Println("---")
	fmt.Println("Budgets | color=#6b7280")
	if len(snapshot.Budgets) == 0 {
		fmt.Println("No budgets configured")
	} else {
		for _, b := range snapshot.Budgets {
			pct := 0.0
			if b.LimitCents > 0 {
				pct = (float64(b.CurrentSpendCents) / float64(b.LimitCents)) * 100.0
			}
			fmt.Printf("%s: %s / %s (%0.0f%%) %s\n", b.Provider, formatUSD(b.CurrentSpendCents), formatUSD(b.LimitCents), pct, progressBar(pct))
		}
	}

	fmt.Println("---")
	fmt.Println("Access | color=#6b7280")
	fmt.Printf("Vault Keys: %d provider(s)\n", len(snapshot.VaultProviders))
	if len(snapshot.RevokedProviders) == 0 {
		fmt.Println("Revoked: none")
	} else {
		fmt.Printf("Revoked: %s\n", strings.Join(snapshot.RevokedProviders, ", "))
	}

	if len(snapshot.Warnings) > 0 {
		fmt.Println("---")
		fmt.Println("Warnings | color=#f59e0b")
		for _, warning := range snapshot.Warnings {
			fmt.Printf("%s\n", warning)
		}
	}

	fmt.Println("---")
	fmt.Println("Actions | color=#6b7280")
	exe := "tokfence"
	if resolved, err := os.Executable(); err == nil {
		exe = resolved
	}
	printSwiftBarAction("Start Daemon", exe, []string{"start", "-d"}, false, true)
	printSwiftBarAction("Stop Daemon", exe, []string{"stop"}, false, true)
	printSwiftBarAction("Kill All Providers", exe, []string{"kill"}, false, true)
	printSwiftBarAction("Restore All Providers", exe, []string{"unkill"}, false, true)
	printSwiftBarAction("Open Live Logs", exe, []string{"log", "-f"}, true, false)
	printSwiftBarAction("Open Stats", exe, []string{"stats", "--period", "today", "--by", "provider"}, true, false)
	printSwiftBarAction("Open Tokfence Data Folder", "open", []string{mustDataDir()}, false, false)
	printSwiftBarAction("Refresh", "echo", []string{"refresh"}, false, true)
}

func printSwiftBarAction(label, bash string, args []string, terminal bool, refresh bool) {
	fmt.Printf("%s | bash=%q", label, bash)
	for i, arg := range args {
		fmt.Printf(" param%d=%q", i+1, arg)
	}
	fmt.Printf(" terminal=%t refresh=%t\n", terminal, refresh)
}

func formatUSD(cents int64) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100.0)
}

func formatTokenCount(tokens int64) string {
	switch {
	case tokens >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(tokens)/1_000_000.0)
	case tokens >= 1_000:
		return fmt.Sprintf("%.1fk", float64(tokens)/1_000.0)
	default:
		return fmt.Sprintf("%d", tokens)
	}
}

func progressBar(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int((pct + 9.99) / 10.0)
	if filled > 10 {
		filled = 10
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
}

type daemonState struct {
	PID       int    `json:"pid"`
	Addr      string `json:"addr"`
	StartedAt string `json:"started_at"`
	UID       int    `json:"uid"`
	Binary    string `json:"binary"`
	CmdNonce  string `json:"cmd_nonce"`
}

func pidFilePath() string {
	return filepath.Join(mustDataDir(), "tokfence.pid")
}

func mustDataDir() string {
	dir, _ := config.ExpandPath("~/.tokfence")
	_ = os.MkdirAll(dir, 0o700)
	_ = os.Chmod(dir, 0o700)
	return dir
}

func writePIDFile(pid int, addr, binary string, uid int, cmdNonce string) error {
	state := daemonState{
		PID:       pid,
		Addr:      addr,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		UID:       uid,
		Binary:    binary,
		CmdNonce:  cmdNonce,
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	pidPath := pidFilePath()
	pidDir := filepath.Dir(pidPath)
	if err := os.MkdirAll(pidDir, 0o700); err != nil {
		return fmt.Errorf("create pid dir: %w", err)
	}
	if err := os.Chmod(pidDir, 0o700); err != nil {
		return fmt.Errorf("set pid dir perms: %w", err)
	}
	if fi, err := os.Lstat(pidPath); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("pid file path is a symlink")
	}
	tmp, err := os.CreateTemp(pidDir, "tokfence.pid.tmp-*")
	if err != nil {
		return fmt.Errorf("create temp pid file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp pid file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temp pid file perms: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp pid file: %w", err)
	}
	if err := os.Rename(tmpPath, pidPath); err != nil {
		return fmt.Errorf("rename pid file: %w", err)
	}
	return nil
}

func readPIDFile() (daemonState, error) {
	data, err := os.ReadFile(pidFilePath())
	if err != nil {
		return daemonState{}, fmt.Errorf("read pid file: %w", err)
	}
	var state daemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return daemonState{}, fmt.Errorf("parse pid file: %w", err)
	}
	if state.PID <= 0 {
		return daemonState{}, errors.New("invalid pid file")
	}
	return state, nil
}

func removePIDFile() {
	_ = os.Remove(pidFilePath())
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func processCommandLine(pid int) string {
	if pid <= 0 {
		return ""
	}
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func processCommandMatchesBinary(pid int, expected string) bool {
	if strings.TrimSpace(expected) == "" {
		return false
	}
	commandLine := processCommandLine(pid)
	if commandLine == "" {
		return false
	}
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		return false
	}
	procName := parts[0]
	if procName == expected || strings.HasSuffix(procName, "/"+filepath.Base(expected)) {
		return true
	}
	if filepath.Base(procName) == filepath.Base(expected) {
		return true
	}
	return false
}

func processCommandContainsNonce(pid int, nonce string) bool {
	if strings.TrimSpace(nonce) == "" {
		return true
	}
	commandLine := processCommandLine(pid)
	if commandLine == "" {
		return false
	}
	if strings.Contains(commandLine, daemonNonceEnv+"="+nonce) {
		return true
	}
	fields := strings.Fields(commandLine)
	for i := 0; i < len(fields); i++ {
		if fields[i] == "--tokfence-daemon-nonce" {
			if i+1 < len(fields) && fields[i+1] == nonce {
				return true
			}
			continue
		}
		if strings.HasPrefix(fields[i], "--tokfence-daemon-nonce=") {
			if strings.TrimPrefix(fields[i], "--tokfence-daemon-nonce=") == nonce {
				return true
			}
		}
	}
	return false
}

func verifyDaemonProcess(state daemonState) error {
	if state.PID <= 0 {
		return errors.New("invalid pid in pid file")
	}
	if !processAlive(state.PID) {
		return fmt.Errorf("no running process for pid %d", state.PID)
	}
	if state.UID != 0 && state.UID != os.Getuid() {
		return errors.New("pid file owner mismatch: refusing to stop foreign process")
	}
	if state.Binary != "" && !processCommandMatchesBinary(state.PID, state.Binary) {
		return errors.New("pid file identity mismatch: refusing to stop foreign process")
	}
	if state.CmdNonce != "" && !processCommandContainsNonce(state.PID, state.CmdNonce) {
		return errors.New("pid file identity mismatch: nonce mismatch")
	}
	return nil
}

func protectedProcessState(state daemonState) (bool, string) {
	if err := verifyDaemonProcess(state); err != nil {
		return false, err.Error()
	}
	return true, ""
}

func isProtectedProcessAlive(state daemonState) bool {
	_, reason := protectedProcessState(state)
	return reason == ""
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func parseWindowStart(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	d, err := time.ParseDuration(raw)
	if err == nil {
		return time.Now().UTC().Add(-d), nil
	}
	if strings.HasSuffix(raw, "d") {
		n, convErr := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		if convErr != nil {
			return time.Time{}, fmt.Errorf("parse --since %q", raw)
		}
		return time.Now().UTC().Add(-time.Duration(n) * 24 * time.Hour), nil
	}
	return time.Time{}, fmt.Errorf("invalid --since format %q", raw)
}

func parseStatsPeriod(raw string) (time.Time, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	now := time.Now().UTC()
	switch raw {
	case "", "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	case "7d", "30d":
		n, _ := strconv.Atoi(strings.TrimSuffix(raw, "d"))
		return now.Add(-time.Duration(n) * 24 * time.Hour), nil
	default:
		if d, err := time.ParseDuration(raw); err == nil {
			return now.Add(-d), nil
		}
		if strings.HasSuffix(raw, "d") {
			n, err := strconv.Atoi(strings.TrimSuffix(raw, "d"))
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid --period %q", raw)
			}
			return now.Add(-time.Duration(n) * 24 * time.Hour), nil
		}
		return time.Time{}, fmt.Errorf("invalid --period %q", raw)
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
