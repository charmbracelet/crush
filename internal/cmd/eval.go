package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/charmbracelet/crush/internal/eval"
	"github.com/spf13/cobra"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Run the golden-trajectory eval harness",
	Long: `Golden-trajectory behavioral regression harness (docs/design/EVAL_HARNESS.md).

The corpus lives under --eval-dir (default ./eval):

  eval/corpus/<id>/trajectory.json   task spec
  eval/corpus/<id>/check.sh          scoring function
  eval/flags.json                    flag projection + defaults
  eval/experiments/<name>.json       paired comparison
  eval/results/                      append-only run records
  eval/bands.json                    characterization state

Each agent run is a 'crush run' subprocess in a fresh materialized
workdir with a pinned HOME/XDG — both arms provably run this build.
Provider credentials come from the eval environment.`,
}

var evalQuarantineCmd = &cobra.Command{
	Use:   "quarantine [trajectory-glob...]",
	Short: "Agent-free check validation pass over the corpus",
	Long: `Validate checks without agent runs: the check must agree with
expect_start_state on the start state, pass on start+reference.patch,
and fail on start+counterexample.patch. Failures quarantine the
trajectory until the check is fixed and re-validated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := evalRunner(cmd)
		if err != nil {
			return err
		}
		defer r.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		sel := args
		if len(sel) == 0 {
			sel = []string{"*"}
		}
		verdicts, err := r.QuarantineCorpus(ctx, sel)
		if err != nil {
			return err
		}
		for _, id := range slices.Sorted(maps.Keys(verdicts)) {
			reason := verdicts[id]
			switch {
			case reason == "":
				fmt.Printf("%s: clean\n", id)
			case strings.HasPrefix(string(reason), "skipped:"):
				fmt.Printf("%s: SKIPPED (%s)\n", id, strings.TrimPrefix(string(reason), "skipped:"))
			default:
				fmt.Printf("%s: QUARANTINED (%s)\n", id, reason)
			}
		}
		return nil
	},
}

var evalCharacterizeCmd = &cobra.Command{
	Use:   "characterize [trajectory-glob...]",
	Short: "Genesis / re-characterization runs under the current default condition",
	RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetInt("runs")
		model, _ := cmd.Flags().GetString("model")
		temp, _ := cmd.Flags().GetFloat64("temperature")
		r, err := evalRunner(cmd)
		if err != nil {
			return err
		}
		defer r.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		sel := args
		if len(sel) == 0 {
			sel = []string{"*"}
		}
		return r.Characterize(ctx, model, &temp, n, sel)
	},
}

var evalRunCmd = &cobra.Command{
	Use:   "run <experiment.json>",
	Short: "Run a paired experiment and print the gate report",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("usage: crush eval run <experiment.json>")
		}
		exp, err := eval.LoadExperiment(args[0])
		if err != nil {
			return err
		}
		r, err := evalRunner(cmd)
		if err != nil {
			return err
		}
		defer r.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		rep, err := r.RunExperiment(ctx, exp)
		if err != nil {
			return err
		}
		alpha := r.Alpha
		if alpha <= 0 {
			alpha = 0.05
		}
		fmt.Print(rep.Summary(alpha))
		if rep.Fired(alpha) {
			return fmt.Errorf("gate fired")
		}
		return nil
	},
}

var evalAnalyzeCmd = &cobra.Command{
	Use:   "analyze <session.db>",
	Short: "Reconstruct per-call gate metrics from a session DB",
	Long: `Run the session-DB sequence analysis standalone — the same pass
ExecuteRun runs before appending a run record. The path may be absolute,
CWD-relative, or eval-dir-relative (a RunRecord session_db value).

Useful beyond eval: it doubles as a debugging tool for real sessions,
which is why --workdir and --session exist. --trajectory supplies the
corpus turns so process-turn boundaries are authoritative instead of
repair-prompt fingerprinted.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		evalDir, _ := cmd.Flags().GetString("eval-dir")
		workdir, _ := cmd.Flags().GetString("workdir")
		sessionID, _ := cmd.Flags().GetString("session")
		trajID, _ := cmd.Flags().GetString("trajectory")

		dbPath := args[0]
		if !filepath.IsAbs(dbPath) {
			if _, err := os.Stat(dbPath); err != nil {
				// session_db paths in run records are eval-dir-relative.
				dbPath = filepath.Join(evalDir, dbPath)
			}
		}

		var turns []string
		if trajID != "" {
			traj, err := eval.LoadTrajectory(filepath.Join(evalDir, "corpus", trajID))
			if err != nil {
				return err
			}
			turns = traj.Task.Turns
		}
		if workdir == "" {
			workdir, _ = os.Getwd()
		}

		goos, _ := cmd.Flags().GetString("goos")
		metrics, err := eval.AnalyzeSessionDB(cmd.Context(), dbPath, eval.AnalyzeOptions{
			SessionID: sessionID,
			Workdir:   workdir,
			Turns:     turns,
			GOOS:      goos,
		})
		if err != nil {
			return err
		}
		out, err := json.MarshalIndent(metrics, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

var evalSmokeCmd = &cobra.Command{
	Use:   "smoke",
	Short: "Smoke tier: strict 0/N collapse check over the stable band",
	RunE: func(cmd *cobra.Command, args []string) error {
		n, _ := cmd.Flags().GetInt("runs")
		model, _ := cmd.Flags().GetString("model")
		temp, _ := cmd.Flags().GetFloat64("temperature")
		r, err := evalRunner(cmd)
		if err != nil {
			return err
		}
		defer r.Close()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		alarms, err := r.Smoke(ctx, model, &temp, n)
		if err != nil {
			return err
		}
		if len(alarms) > 0 {
			return fmt.Errorf("smoke collapse: %v", alarms)
		}
		fmt.Println("smoke: clean")
		return nil
	},
}

func evalRunner(cmd *cobra.Command) (*eval.Runner, error) {
	dir, _ := cmd.Flags().GetString("eval-dir")
	return &eval.Runner{EvalDir: dir}, nil
}

func init() {
	evalCmd.PersistentFlags().String("eval-dir", eval.DefaultEvalDir, "eval root directory")
	evalCharacterizeCmd.Flags().IntP("runs", "n", eval.GenesisRuns, "runs per trajectory")
	evalCharacterizeCmd.Flags().StringP("model", "m", "", "model pin (provider/model)")
	evalCharacterizeCmd.Flags().Float64("temperature", 0, "sampling temperature")
	evalSmokeCmd.Flags().IntP("runs", "n", 5, "runs per stable trajectory")
	evalSmokeCmd.Flags().StringP("model", "m", "", "model pin (provider/model)")
	evalSmokeCmd.Flags().Float64("temperature", 0, "sampling temperature")
	_ = evalCharacterizeCmd.MarkFlagRequired("model")
	_ = evalSmokeCmd.MarkFlagRequired("model")
	evalAnalyzeCmd.Flags().String("workdir", "", "run working dir for normalizing relative call paths (default: CWD)")
	evalAnalyzeCmd.Flags().String("session", "", "session ID to analyze (default: latest parent session)")
	evalAnalyzeCmd.Flags().String("trajectory", "", "corpus trajectory ID — supplies turns for process-turn segmentation")
	evalAnalyzeCmd.Flags().String("goos", "", "OS whose path conventions produced the artifact (default: this machine)")
	evalCmd.AddCommand(evalQuarantineCmd, evalCharacterizeCmd, evalRunCmd, evalSmokeCmd, evalAnalyzeCmd)
}
