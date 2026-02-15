package main

import (
	"context"
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func newStartCommand() *cobra.Command {
	var daemonize bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start tokfence daemon",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if daemonize && os.Getenv("TOKFENCE_BACKGROUND") != "1" {
				return spawnBackground(cfg)
			}
			return runForeground(cfg)
		},
	}
	cmd.Flags().BoolVarP(&daemonize, "daemon", "d", false, "run in background")
	return cmd
}

func runForeground(cfg config.Config) error {
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
	if err := writePIDFile(os.Getpid(), server.Addr()); err != nil {
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

func spawnBackground(cfg config.Config) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	args := []string{"start", "--config", configPath}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "TOKFENCE_BACKGROUND=1")
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
			if err := syscall.Kill(state.PID, syscall.SIGTERM); err != nil {
				if errors.Is(err, os.ErrProcessDone) {
					removePIDFile()
					return nil
				}
				return fmt.Errorf("stop daemon: %w", err)
			}
			deadline := time.Now().Add(30 * time.Second)
			for time.Now().Before(deadline) {
				alive := processAlive(state.PID)
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
			running := processAlive(state.PID)
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

type daemonState struct {
	PID       int    `json:"pid"`
	Addr      string `json:"addr"`
	StartedAt string `json:"started_at"`
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

func writePIDFile(pid int, addr string) error {
	state := daemonState{PID: pid, Addr: addr, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.WriteFile(pidFilePath(), data, 0o600); err != nil {
		return fmt.Errorf("write pid file: %w", err)
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
