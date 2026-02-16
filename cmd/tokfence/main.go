package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	rootCmd.AddCommand(newWatchCommand())
	rootCmd.AddCommand(newProviderCommand())
	rootCmd.AddCommand(newSetupCommand())
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
			cfg, cfgErr := config.Load(configPath)
			if cfgErr != nil {
				return cfgErr
			}
			state, err := readPIDFile()
			if err != nil {
				addr := fmt.Sprintf("%s:%d", cfg.Daemon.Host, cfg.Daemon.Port)
				conn, probeErr := net.DialTimeout("tcp", addr, 500*time.Millisecond)
				if probeErr == nil {
					_ = conn.Close()
					if outputJSON {
						return printJSON(map[string]any{
							"running": true,
							"addr":    addr,
							"managed": false,
							"source":  "port_probe",
						})
					}
					fmt.Printf("tokfence daemon is running\nAddr: %s\nManaged: no (pid file missing)\n", addr)
					return nil
				}
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

type watchUsageTotals struct {
	RequestCount      int64  `json:"request_count"`
	InputTokens       int64  `json:"input_tokens"`
	OutputTokens      int64  `json:"output_tokens"`
	CostCents         int64  `json:"cost_cents"`
	CostUSD           string `json:"cost_usd"`
	LastRequestAt     string `json:"last_request_at,omitempty"`
	RequestCountKnown bool   `json:"request_count_known,omitempty"`
	InputTokensKnown  bool   `json:"input_tokens_known,omitempty"`
	OutputTokensKnown bool   `json:"output_tokens_known,omitempty"`
	CostKnown         bool   `json:"cost_known,omitempty"`
}

type watchProviderReport struct {
	Provider          string           `json:"provider"`
	Status            string           `json:"status"`
	Message           string           `json:"message,omitempty"`
	CheckedAt         time.Time        `json:"checked_at"`
	RemoteSource      string           `json:"remote_source,omitempty"`
	Local             watchUsageTotals `json:"local"`
	Remote            watchUsageTotals `json:"remote"`
	DeltaRequests     int64            `json:"delta_requests"`
	DeltaInputTokens  int64            `json:"delta_input_tokens"`
	DeltaOutputTokens int64            `json:"delta_output_tokens"`
	DeltaCostCents    int64            `json:"delta_cost_cents"`
	DeltaCostUSD      string           `json:"delta_cost_usd"`
	IdleForSeconds    int64            `json:"idle_for_seconds,omitempty"`
	IdleLeak          bool             `json:"idle_leak"`
	LeakSuspected     bool             `json:"leak_suspected"`
	AutoRevoked       bool             `json:"auto_revoked"`
	FetchError        string           `json:"fetch_error,omitempty"`
}

type watchPollReport struct {
	CheckedAt       time.Time             `json:"checked_at"`
	Period          string                `json:"period"`
	ThresholdUSD    float64               `json:"threshold_usd"`
	ThresholdTokens int64                 `json:"threshold_tokens"`
	ThresholdReqs   int64                 `json:"threshold_requests"`
	IdleWindowSec   int64                 `json:"idle_window_seconds"`
	Providers       []watchProviderReport `json:"providers"`
	Alerts          int                   `json:"alerts"`
}

type remoteUsageEndpoint struct {
	name string
	url  string
}

func newWatchCommand() *cobra.Command {
	var providers []string
	var period string
	var interval time.Duration
	var thresholdUSD float64
	var thresholdTokens int64
	var thresholdRequests int64
	var idleWindow time.Duration
	var once bool
	var autoRevoke bool
	var customUsageEndpoints []string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Detect potential API key misuse by reconciling provider usage vs local proxy logs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			since, err := parseStatsPeriod(period)
			if err != nil {
				return err
			}
			if interval < 10*time.Second {
				return errors.New("interval must be >= 10s")
			}
			if idleWindow < time.Minute {
				return errors.New("idle-window must be >= 1m")
			}
			if thresholdUSD < 0 {
				return errors.New("threshold-usd must be >= 0")
			}
			if thresholdTokens < 0 {
				return errors.New("threshold-tokens must be >= 0")
			}
			if thresholdRequests < 0 {
				return errors.New("threshold-requests must be >= 0")
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

			resolvedProviders, err := resolveWatchProviders(providers, cfg, v)
			if err != nil {
				return err
			}
			if len(resolvedProviders) == 0 {
				return errors.New("no providers selected for watch (configure at least one key in vault)")
			}

			customEndpoints, err := parseCustomUsageEndpointFlags(customUsageEndpoints)
			if err != nil {
				return err
			}
			client := &http.Client{Timeout: 15 * time.Second}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			previousRemote := map[string]watchUsageTotals{}
			thresholdCents := int64(thresholdUSD * 100.0)
			runOnce := func() (watchPollReport, error) {
				report := watchPollReport{
					CheckedAt:       time.Now().UTC(),
					Period:          period,
					ThresholdUSD:    thresholdUSD,
					ThresholdTokens: thresholdTokens,
					ThresholdReqs:   thresholdRequests,
					IdleWindowSec:   int64(idleWindow.Seconds()),
					Providers:       make([]watchProviderReport, 0, len(resolvedProviders)),
				}
				for _, provider := range resolvedProviders {
					providerReport := watchProviderReport{
						Provider:     provider,
						CheckedAt:    report.CheckedAt,
						Status:       "ok",
						DeltaCostUSD: formatUSD(0),
						Local: watchUsageTotals{
							CostUSD: formatUSD(0),
						},
						Remote: watchUsageTotals{
							CostUSD: formatUSD(0),
						},
					}
					localUsage, localErr := loadLocalWatchUsage(ctx, store, provider, since)
					if localErr != nil {
						providerReport.Status = "local_error"
						providerReport.FetchError = localErr.Error()
						providerReport.Message = "failed to load local usage"
						report.Providers = append(report.Providers, providerReport)
						report.Alerts++
						continue
					}
					providerReport.Local = localUsage

					remoteUsage, remoteSource, remoteErr := fetchRemoteWatchUsage(ctx, client, cfg, v, provider, since, report.CheckedAt, customEndpoints[provider])
					if remoteErr != nil {
						providerReport.Status = "remote_error"
						providerReport.FetchError = remoteErr.Error()
						providerReport.Message = "failed to fetch provider usage endpoint"
						report.Providers = append(report.Providers, providerReport)
						report.Alerts++
						continue
					}
					providerReport.Remote = remoteUsage
					providerReport.RemoteSource = remoteSource

					providerReport.DeltaRequests = remoteUsage.RequestCount - localUsage.RequestCount
					providerReport.DeltaInputTokens = remoteUsage.InputTokens - localUsage.InputTokens
					providerReport.DeltaOutputTokens = remoteUsage.OutputTokens - localUsage.OutputTokens
					providerReport.DeltaCostCents = remoteUsage.CostCents - localUsage.CostCents
					providerReport.DeltaCostUSD = formatUSD(providerReport.DeltaCostCents)

					leakByCost := remoteUsage.CostKnown && providerReport.DeltaCostCents > thresholdCents
					totalLocalTokens := localUsage.InputTokens + localUsage.OutputTokens
					totalRemoteTokens := remoteUsage.InputTokens + remoteUsage.OutputTokens
					leakByTokens := (remoteUsage.InputTokensKnown || remoteUsage.OutputTokensKnown) && (totalRemoteTokens-totalLocalTokens) > thresholdTokens
					leakByReqs := remoteUsage.RequestCountKnown && providerReport.DeltaRequests > thresholdRequests

					if last, ok := parseOptionalTimestamp(localUsage.LastRequestAt); ok {
						idleFor := report.CheckedAt.Sub(last)
						if idleFor > 0 {
							providerReport.IdleForSeconds = int64(idleFor.Seconds())
						}
						if prev, exists := previousRemote[provider]; exists && idleFor >= idleWindow {
							remoteMoved := (remoteUsage.CostCents > prev.CostCents) ||
								(remoteUsage.InputTokens > prev.InputTokens) ||
								(remoteUsage.OutputTokens > prev.OutputTokens) ||
								(remoteUsage.RequestCount > prev.RequestCount)
							if remoteMoved {
								providerReport.IdleLeak = true
							}
						}
					}

					providerReport.LeakSuspected = leakByCost || leakByTokens || leakByReqs || providerReport.IdleLeak
					if providerReport.LeakSuspected {
						providerReport.Status = "alert"
						providerReport.Message = "remote usage exceeds local proxy logs"
						report.Alerts++
						if autoRevoke {
							if err := store.SetProviderRevoked(ctx, provider, true); err == nil {
								providerReport.AutoRevoked = true
								providerReport.Message = "remote usage exceeds local proxy logs (provider auto-revoked)"
							}
						}
					}

					previousRemote[provider] = remoteUsage
					report.Providers = append(report.Providers, providerReport)
				}
				return report, nil
			}

			if once {
				report, err := runOnce()
				if err != nil {
					return err
				}
				if outputJSON {
					return printJSON(report)
				}
				renderWatchReport(report)
				return nil
			}

			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				report, err := runOnce()
				if err != nil {
					return err
				}
				if outputJSON {
					if err := printJSON(report); err != nil {
						return err
					}
				} else {
					renderWatchReport(report)
				}
				select {
				case <-ctx.Done():
					return nil
				case <-ticker.C:
				}
			}
		},
	}
	cmd.Flags().StringSliceVar(&providers, "provider", nil, "provider(s) to watch (repeat flag)")
	cmd.Flags().StringVar(&period, "period", "24h", "comparison period (today, 24h, 7d, 30d)")
	cmd.Flags().DurationVar(&interval, "interval", 15*time.Minute, "polling interval")
	cmd.Flags().Float64Var(&thresholdUSD, "threshold-usd", 1.0, "alert when remote cost exceeds local by this USD")
	cmd.Flags().Int64Var(&thresholdTokens, "threshold-tokens", 1000, "alert when remote tokens exceed local by this amount")
	cmd.Flags().Int64Var(&thresholdRequests, "threshold-requests", 1, "alert when remote request count exceeds local by this amount")
	cmd.Flags().DurationVar(&idleWindow, "idle-window", 30*time.Minute, "idle duration used for idle leak detection")
	cmd.Flags().BoolVar(&once, "once", false, "run one check and exit")
	cmd.Flags().BoolVar(&autoRevoke, "auto-revoke", false, "revoke provider automatically when leak is suspected")
	cmd.Flags().StringArrayVar(&customUsageEndpoints, "usage-endpoint", nil, "override usage endpoint as provider=url (repeat flag)")
	return cmd
}

func newProviderCommand() *cobra.Command {
	providerCmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage provider endpoints",
	}

	providerCmd.AddCommand(&cobra.Command{
		Use:   "set <provider> <upstream>",
		Short: "Set provider upstream endpoint",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.ToLower(strings.TrimSpace(args[0]))
			if err := vault.ValidateProvider(provider); err != nil {
				return err
			}

			upstream := strings.TrimSpace(args[1])
			if upstream == "" {
				return errors.New("upstream endpoint is required")
			}
			parsed, err := url.Parse(upstream)
			if err != nil {
				return fmt.Errorf("parse upstream URL: %w", err)
			}
			if parsed.Scheme == "" || parsed.Host == "" {
				return errors.New("upstream must be an absolute URL")
			}
			if parsed.Scheme != "https" && parsed.Scheme != "http" {
				return errors.New("upstream URL scheme must be http or https")
			}
			upstream = strings.TrimRight(parsed.String(), "/")

			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if cfg.Providers == nil {
				cfg.Providers = map[string]config.ProviderConfig{}
			}
			current := cfg.Providers[provider]
			current.Upstream = upstream
			cfg.Providers[provider] = current

			if err := config.Save(configPath, cfg); err != nil {
				return err
			}

			if outputJSON {
				return printJSON(map[string]any{
					"provider": provider,
					"upstream": upstream,
					"status":   "set",
				})
			}
			fmt.Printf("provider %s upstream set to %s\n", provider, upstream)
			return nil
		},
	})

	return providerCmd
}

func newSetupCommand() *cobra.Command {
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Generate setup snippets for supported tools",
	}

	var provider string
	var runTest bool
	openclawCmd := &cobra.Command{
		Use:   "openclaw",
		Short: "Generate OpenClaw config.yaml snippet",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}

			selectedProvider := strings.ToLower(strings.TrimSpace(provider))
			if selectedProvider == "" {
				selectedProvider = "openai"
			}
			if err := vault.ValidateProvider(selectedProvider); err != nil {
				return err
			}
			if _, ok := cfg.Providers[selectedProvider]; !ok {
				return fmt.Errorf("provider %q is not configured; run `tokfence provider set %s <upstream-url>` first", selectedProvider, selectedProvider)
			}

			baseURL := fmt.Sprintf("http://%s:%d/%s", cfg.Daemon.Host, cfg.Daemon.Port, selectedProvider)
			configLine := fmt.Sprintf("base_url: %q", baseURL)

			testResult := map[string]any{}
			if runTest {
				addr := net.JoinHostPort(cfg.Daemon.Host, strconv.Itoa(cfg.Daemon.Port))
				conn, dialErr := net.DialTimeout("tcp", addr, 2*time.Second)
				daemonReachable := dialErr == nil
				if conn != nil {
					_ = conn.Close()
				}
				testResult["daemon_reachable"] = daemonReachable

				v, vErr := vault.NewDefault(vault.Options{})
				if vErr != nil {
					return fmt.Errorf("init vault for setup test: %w", vErr)
				}
				providers, listErr := v.List(context.Background())
				if listErr != nil {
					return fmt.Errorf("list vault providers for setup test: %w", listErr)
				}
				hasKey := false
				for _, p := range providers {
					if strings.EqualFold(p, selectedProvider) {
						hasKey = true
						break
					}
				}
				testResult["provider_has_key"] = hasKey

				if !daemonReachable || !hasKey {
					if outputJSON {
						return printJSON(map[string]any{
							"tool":        "openclaw",
							"provider":    selectedProvider,
							"base_url":    baseURL,
							"config_line": configLine,
							"test":        testResult,
							"ready":       false,
						})
					}
					if !daemonReachable {
						return fmt.Errorf("setup test failed: daemon is not reachable on %s", addr)
					}
					return fmt.Errorf("setup test failed: no key configured for provider %q", selectedProvider)
				}
			}

			if outputJSON {
				payload := map[string]any{
					"tool":        "openclaw",
					"provider":    selectedProvider,
					"base_url":    baseURL,
					"config_line": configLine,
					"ready":       !runTest || (testResult["daemon_reachable"] == true && testResult["provider_has_key"] == true),
				}
				if runTest {
					payload["test"] = testResult
				}
				return printJSON(payload)
			}

			fmt.Println("# OpenClaw config.yaml")
			fmt.Println(configLine)
			fmt.Println()
			fmt.Printf("# Provider: %s\n", selectedProvider)
			fmt.Printf("# Proxy URL: %s\n", baseURL)
			if runTest {
				fmt.Printf("# Test daemon reachable: %v\n", testResult["daemon_reachable"])
				fmt.Printf("# Test key configured: %v\n", testResult["provider_has_key"])
			}
			return nil
		},
	}
	openclawCmd.Flags().StringVar(&provider, "provider", "openai", "provider to target in the generated OpenClaw config")
	openclawCmd.Flags().BoolVar(&runTest, "test", false, "verify daemon reachability and key availability")

	setupCmd.AddCommand(openclawCmd)
	return setupCmd
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

func resolveWatchProviders(selected []string, cfg config.Config, v vault.Vault) ([]string, error) {
	normalized := make([]string, 0, len(selected))
	seen := map[string]struct{}{}
	for _, raw := range selected {
		provider := strings.ToLower(strings.TrimSpace(raw))
		if provider == "" {
			continue
		}
		if err := vault.ValidateProvider(provider); err != nil {
			return nil, err
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		normalized = append(normalized, provider)
		seen[provider] = struct{}{}
	}
	if len(normalized) > 0 {
		sort.Strings(normalized)
		return normalized, nil
	}

	vaultProviders, err := v.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("list vault providers: %w", err)
	}
	for _, provider := range vaultProviders {
		provider = strings.ToLower(strings.TrimSpace(provider))
		if provider == "" {
			continue
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		normalized = append(normalized, provider)
		seen[provider] = struct{}{}
	}

	if len(normalized) == 0 {
		for provider := range cfg.Providers {
			if _, ok := seen[provider]; ok {
				continue
			}
			normalized = append(normalized, provider)
			seen[provider] = struct{}{}
		}
	}
	sort.Strings(normalized)
	return normalized, nil
}

func parseCustomUsageEndpointFlags(values []string) (map[string]string, error) {
	result := map[string]string{}
	for _, value := range values {
		raw := strings.TrimSpace(value)
		if raw == "" {
			continue
		}
		idx := strings.Index(raw, "=")
		if idx <= 0 || idx >= len(raw)-1 {
			return nil, fmt.Errorf("invalid --usage-endpoint %q (expected provider=url)", value)
		}
		provider := strings.ToLower(strings.TrimSpace(raw[:idx]))
		endpoint := strings.TrimSpace(raw[idx+1:])
		if err := vault.ValidateProvider(provider); err != nil {
			return nil, err
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" {
			return nil, fmt.Errorf("invalid usage endpoint for %s: %q", provider, endpoint)
		}
		result[provider] = endpoint
	}
	return result, nil
}

func loadLocalWatchUsage(ctx context.Context, store *logger.LogStore, provider string, since time.Time) (watchUsageTotals, error) {
	out := watchUsageTotals{
		CostUSD:           formatUSD(0),
		RequestCountKnown: true,
		InputTokensKnown:  true,
		OutputTokensKnown: true,
		CostKnown:         true,
	}

	rows, err := store.Stats(ctx, logger.StatsFilter{Provider: provider, Since: since, By: "provider"})
	if err != nil {
		return out, err
	}
	if len(rows) > 0 {
		row := rows[0]
		out.RequestCount = int64(row.RequestCount)
		out.InputTokens = row.InputTokens
		out.OutputTokens = row.OutputTokens
		out.CostCents = row.EstimatedCostCents
		out.CostUSD = formatUSD(row.EstimatedCostCents)
	}

	latest, err := store.ListRequests(ctx, logger.QueryFilter{Limit: 1, Provider: provider})
	if err != nil {
		return out, err
	}
	if len(latest) > 0 {
		out.LastRequestAt = latest[0].Timestamp.UTC().Format(time.RFC3339)
	}
	return out, nil
}

func fetchRemoteWatchUsage(
	ctx context.Context,
	client *http.Client,
	cfg config.Config,
	v vault.Vault,
	provider string,
	since, now time.Time,
	customEndpoint string,
) (watchUsageTotals, string, error) {
	key, err := v.Get(ctx, provider)
	if err != nil {
		return watchUsageTotals{}, "", fmt.Errorf("load vault key: %w", err)
	}

	providerCfg, ok := cfg.Providers[provider]
	if !ok {
		providerCfg = config.ProviderConfig{}
	}
	if strings.TrimSpace(customEndpoint) == "" && strings.TrimSpace(providerCfg.Upstream) == "" {
		return watchUsageTotals{}, "", fmt.Errorf("provider %q has no configured upstream", provider)
	}

	endpoints, err := usageEndpointsForProvider(provider, providerCfg.Upstream, since, now, customEndpoint)
	if err != nil {
		return watchUsageTotals{}, "", err
	}
	if len(endpoints) == 0 {
		return watchUsageTotals{}, "", fmt.Errorf("provider %q has no supported usage endpoint", provider)
	}

	failures := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		usage, err := fetchUsageEndpoint(ctx, client, provider, key, providerCfg.ExtraHeaders, endpoint.url)
		if err != nil {
			failures = append(failures, endpoint.name+": "+err.Error())
			continue
		}
		return usage, endpoint.name, nil
	}
	return watchUsageTotals{}, "", errors.New(strings.Join(failures, " | "))
}

func usageEndpointsForProvider(provider, upstream string, since, now time.Time, customEndpoint string) ([]remoteUsageEndpoint, error) {
	if strings.TrimSpace(customEndpoint) != "" {
		return []remoteUsageEndpoint{{name: "custom", url: customEndpoint}}, nil
	}

	startDate := since.UTC().Format("2006-01-02")
	endDate := now.UTC().Format("2006-01-02")
	startEpoch := since.UTC().Unix()
	endEpoch := now.UTC().Unix()

	switch provider {
	case "openai":
		u0, err := buildUsageURL(upstream, "/v1/organization/costs", url.Values{
			"start_time": []string{strconv.FormatInt(startEpoch, 10)},
			"end_time":   []string{strconv.FormatInt(endEpoch, 10)},
		})
		if err != nil {
			return nil, err
		}
		u1, err := buildUsageURL(upstream, "/v1/dashboard/billing/usage", url.Values{
			"start_date": []string{startDate},
			"end_date":   []string{endDate},
		})
		if err != nil {
			return nil, err
		}
		u2, err := buildUsageURL(upstream, "/v1/organization/usage/completions", url.Values{
			"start_time": []string{strconv.FormatInt(startEpoch, 10)},
			"end_time":   []string{strconv.FormatInt(endEpoch, 10)},
		})
		if err != nil {
			return nil, err
		}
		return []remoteUsageEndpoint{
			{name: "openai_org_costs", url: u0},
			{name: "openai_dashboard_usage", url: u1},
			{name: "openai_org_usage", url: u2},
		}, nil
	case "anthropic":
		u0, err := buildUsageURL(upstream, "/v1/organizations/cost_report", url.Values{
			"starting_at": []string{since.UTC().Format(time.RFC3339)},
			"ending_at":   []string{now.UTC().Format(time.RFC3339)},
		})
		if err != nil {
			return nil, err
		}
		u1, err := buildUsageURL(upstream, "/v1/organizations/usage_report/messages", url.Values{
			"starting_at": []string{since.UTC().Format(time.RFC3339)},
			"ending_at":   []string{now.UTC().Format(time.RFC3339)},
			"limit":       []string{"100"},
		})
		if err != nil {
			return nil, err
		}
		u2, err := buildUsageURL(upstream, "/v1/usage", url.Values{
			"start_date": []string{startDate},
			"end_date":   []string{endDate},
		})
		if err != nil {
			return nil, err
		}
		return []remoteUsageEndpoint{
			{name: "anthropic_org_cost_report", url: u0},
			{name: "anthropic_org_usage_messages", url: u1},
			{name: "anthropic_usage_dates", url: u2},
		}, nil
	default:
		return nil, fmt.Errorf("provider %q usage endpoint not yet supported (set --usage-endpoint %s=<url>)", provider, provider)
	}
}

func buildUsageURL(base, extraPath string, query url.Values) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("parse upstream %q: %w", base, err)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return "", fmt.Errorf("upstream must be absolute URL, got %q", base)
	}

	trimmedBase := strings.TrimRight(parsed.Path, "/")
	trimmedExtra := strings.TrimLeft(extraPath, "/")
	if trimmedExtra != "" {
		if trimmedBase == "" {
			parsed.Path = "/" + trimmedExtra
		} else {
			parsed.Path = trimmedBase + "/" + trimmedExtra
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func fetchUsageEndpoint(
	ctx context.Context,
	client *http.Client,
	provider, key string,
	extraHeaders map[string]string,
	targetURL string,
) (watchUsageTotals, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return watchUsageTotals{}, err
	}
	req.Header.Set("Accept", "application/json")
	if err := applyUsageAuth(req.Header, provider, key); err != nil {
		return watchUsageTotals{}, err
	}
	for hk, hv := range extraHeaders {
		if strings.TrimSpace(hk) == "" {
			continue
		}
		req.Header.Set(hk, hv)
	}

	resp, err := client.Do(req)
	if err != nil {
		return watchUsageTotals{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return watchUsageTotals{}, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return watchUsageTotals{}, fmt.Errorf("http %d: %s", resp.StatusCode, clippedBody(body, 240))
	}
	return parseRemoteUsageTotals(body)
}

func applyUsageAuth(headers http.Header, provider, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("missing API key for provider %q", provider)
	}
	switch provider {
	case "anthropic":
		headers.Set("x-api-key", key)
		headers.Set("anthropic-version", "2023-06-01")
	case "google":
		headers.Set("x-goog-api-key", key)
	default:
		headers.Set("Authorization", "Bearer "+key)
	}
	return nil
}

func parseRemoteUsageTotals(body []byte) (watchUsageTotals, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return watchUsageTotals{}, fmt.Errorf("decode usage response: %w", err)
	}

	acc := usageAccumulator{
		costCentsTotals:   []float64{},
		costCentsParts:    []float64{},
		costUSDTotals:     []float64{},
		costUSDParts:      []float64{},
		inputTokenTotals:  []float64{},
		inputTokenParts:   []float64{},
		outputTokenTotals: []float64{},
		outputTokenParts:  []float64{},
		requestTotals:     []float64{},
		requestParts:      []float64{},
	}
	collectUsageValues(payload, &acc)

	var out watchUsageTotals

	if len(acc.costCentsTotals) > 0 {
		out.CostKnown = true
		out.CostCents = int64(maxFloatSlice(acc.costCentsTotals))
	} else if len(acc.costUSDTotals) > 0 {
		out.CostKnown = true
		out.CostCents = int64(maxFloatSlice(acc.costUSDTotals) * 100.0)
	} else if len(acc.costCentsParts) > 0 || len(acc.costUSDParts) > 0 {
		out.CostKnown = true
		out.CostCents = int64(sumFloatSlice(acc.costCentsParts) + (sumFloatSlice(acc.costUSDParts) * 100.0))
	}
	out.CostUSD = formatUSD(out.CostCents)

	if len(acc.inputTokenTotals) > 0 {
		out.InputTokensKnown = true
		out.InputTokens = int64(maxFloatSlice(acc.inputTokenTotals))
	} else if len(acc.inputTokenParts) > 0 {
		out.InputTokensKnown = true
		out.InputTokens = int64(sumFloatSlice(acc.inputTokenParts))
	}

	if len(acc.outputTokenTotals) > 0 {
		out.OutputTokensKnown = true
		out.OutputTokens = int64(maxFloatSlice(acc.outputTokenTotals))
	} else if len(acc.outputTokenParts) > 0 {
		out.OutputTokensKnown = true
		out.OutputTokens = int64(sumFloatSlice(acc.outputTokenParts))
	}

	if len(acc.requestTotals) > 0 {
		out.RequestCountKnown = true
		out.RequestCount = int64(maxFloatSlice(acc.requestTotals))
	} else if len(acc.requestParts) > 0 {
		out.RequestCountKnown = true
		out.RequestCount = int64(sumFloatSlice(acc.requestParts))
	}

	if !out.CostKnown && !out.InputTokensKnown && !out.OutputTokensKnown && !out.RequestCountKnown {
		return out, errors.New("usage response did not expose recognizable totals")
	}
	return out, nil
}

type usageAccumulator struct {
	costCentsTotals   []float64
	costCentsParts    []float64
	costUSDTotals     []float64
	costUSDParts      []float64
	inputTokenTotals  []float64
	inputTokenParts   []float64
	outputTokenTotals []float64
	outputTokenParts  []float64
	requestTotals     []float64
	requestParts      []float64
}

func collectUsageValues(value any, acc *usageAccumulator) {
	switch v := value.(type) {
	case map[string]any:
		for rawKey, rawVal := range v {
			key := normalizeUsageKey(rawKey)
			if number, ok := parseJSONNumber(rawVal); ok {
				switch key {
				case "total_usage", "total_usage_cents", "total_cost_cents", "total_spend_cents", "usage_cents":
					acc.costCentsTotals = append(acc.costCentsTotals, number)
				case "cost_cents", "amount_cents":
					acc.costCentsParts = append(acc.costCentsParts, number)
				case "total_cost", "total_usd":
					acc.costUSDTotals = append(acc.costUSDTotals, number)
				case "cost", "amount", "usd", "cost_usd", "amount_usd":
					acc.costUSDParts = append(acc.costUSDParts, number)
				case "total_input_tokens", "total_prompt_tokens":
					acc.inputTokenTotals = append(acc.inputTokenTotals, number)
				case "input_tokens", "prompt_tokens", "billable_input_tokens", "cache_read_input_tokens", "cache_creation_input_tokens":
					acc.inputTokenParts = append(acc.inputTokenParts, number)
				case "total_output_tokens", "total_completion_tokens":
					acc.outputTokenTotals = append(acc.outputTokenTotals, number)
				case "output_tokens", "completion_tokens", "billable_output_tokens":
					acc.outputTokenParts = append(acc.outputTokenParts, number)
				case "total_requests", "request_count":
					acc.requestTotals = append(acc.requestTotals, number)
				case "requests", "num_requests":
					acc.requestParts = append(acc.requestParts, number)
				}
			}
			collectUsageValues(rawVal, acc)
		}
	case []any:
		for _, item := range v {
			collectUsageValues(item, acc)
		}
	}
}

func normalizeUsageKey(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	return normalized
}

func parseJSONNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	case json.Number:
		if n, err := v.Float64(); err == nil {
			return n, true
		}
	case string:
		clean := strings.TrimSpace(strings.ReplaceAll(v, ",", ""))
		if clean == "" {
			return 0, false
		}
		if n, err := strconv.ParseFloat(clean, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

func maxFloatSlice(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func sumFloatSlice(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum
}

func clippedBody(body []byte, max int) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func parseOptionalTimestamp(raw string) (time.Time, bool) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func renderWatchReport(report watchPollReport) {
	fmt.Printf("[%s] tokfence watch period=%s alerts=%d\n", report.CheckedAt.Local().Format(time.RFC3339), report.Period, report.Alerts)
	for _, provider := range report.Providers {
		totalDeltaTokens := provider.DeltaInputTokens + provider.DeltaOutputTokens
		status := strings.ToUpper(provider.Status)
		fmt.Printf("- %s status=%s local=%s remote=%s delta=%s req_delta=%d token_delta=%d\n",
			provider.Provider,
			status,
			provider.Local.CostUSD,
			provider.Remote.CostUSD,
			provider.DeltaCostUSD,
			provider.DeltaRequests,
			totalDeltaTokens,
		)
		if provider.RemoteSource != "" {
			fmt.Printf("  source=%s\n", provider.RemoteSource)
		}
		if provider.IdleForSeconds > 0 {
			fmt.Printf("  idle_for=%ds\n", provider.IdleForSeconds)
		}
		if provider.AutoRevoked {
			fmt.Println("  action=provider revoked")
		}
		if provider.Message != "" {
			fmt.Printf("  note=%s\n", provider.Message)
		}
		if provider.FetchError != "" {
			fmt.Printf("  error=%s\n", provider.FetchError)
		}
	}
	fmt.Println()
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
