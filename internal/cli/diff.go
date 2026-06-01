package cli

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func newDiffCommand() *cobra.Command {
	var (
		threshold float64
		noColor   bool
	)
	cmd := &cobra.Command{
		Use:   "diff <a.json> <b.json>",
		Short: "Compare two BenchBuddy reports (cluster A vs B, or before vs after)",
		Long: `diff loads two saved reports and shows, per task and metric, the change from A → B.
Regressions are flagged when the change exceeds the threshold (default 10%).
The direction of "better" is inferred per metric: latencies, errors and durations
are better lower; iops, ops/s, bandwidth, samples are better higher.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadReport(args[0])
			if err != nil {
				return err
			}
			b, err := loadReport(args[1])
			if err != nil {
				return err
			}
			return renderDiff(cmd.OutOrStdout(), a, b, args[0], args[1], threshold, !noColor)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 10.0, "regression / improvement threshold in percent")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable ANSI colors")
	return cmd
}

// ANSI escape helpers.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiGrey   = "\033[90m"
)

type tint struct{ on bool }

func (c tint) wrap(code, s string) string {
	if !c.on {
		return s
	}
	return code + s + ansiReset
}

func renderDiff(w io.Writer, a, b runresult.Report, pathA, pathB string, threshold float64, color bool) error {
	c := tint{on: color}

	// Header.
	fmt.Fprintln(w, c.wrap(ansiBold, "BenchBuddy Diff"))
	fmt.Fprintf(w, "  A: %s  run=%s  k8s=%s  cni=%s  nodes=%d\n", pathA, a.Meta.RunID, a.Meta.K8sVersion, a.Meta.CNI, a.Meta.NodeCount)
	fmt.Fprintf(w, "  B: %s  run=%s  k8s=%s  cni=%s  nodes=%d\n", pathB, b.Meta.RunID, b.Meta.K8sVersion, b.Meta.CNI, b.Meta.NodeCount)
	fmt.Fprintf(w, "  threshold: ±%.1f%%\n\n", threshold)

	// Index by taskID.
	indexA := indexResults(a.Results)
	indexB := indexResults(b.Results)

	taskIDs := unionKeys(indexA, indexB)
	sort.Strings(taskIDs)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  TASK\tMETRIC\tA\tB\tΔ\tΔ%\tVERDICT")

	var regressions, improvements, neutrals int
	for _, id := range taskIDs {
		ra, okA := indexA[id]
		rb, okB := indexB[id]
		switch {
		case !okA:
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t\t\t%s\n", id, "—", "—", c.wrap(ansiDim, "(absent)"), c.wrap(ansiBlue, "ONLY IN B"))
			continue
		case !okB:
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t\t\t%s\n", id, "—", c.wrap(ansiDim, "(absent)"), "—", c.wrap(ansiBlue, "ONLY IN A"))
			continue
		}
		// Both present — compare per-metric.
		metricKeys := unionMetricKeys(ra.Metrics, rb.Metrics)
		sort.Strings(metricKeys)
		if len(metricKeys) == 0 {
			// Compare statuses if no metrics (e.g. both Failed).
			va, vb := string(ra.Status), string(rb.Status)
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t\t\t%s\n", id, "status", va, vb, statusVerdict(c, ra.Status, rb.Status))
			continue
		}
		for _, mk := range metricKeys {
			ma, okMA := ra.Metrics[mk]
			mb, okMB := rb.Metrics[mk]
			switch {
			case !okMA:
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s %s\t\t\t%s\n", id, mk, "—", formatNum(mb.Value), mb.Unit, c.wrap(ansiBlue, "ONLY IN B"))
			case !okMB:
				fmt.Fprintf(tw, "  %s\t%s\t%s %s\t%s\t\t\t%s\n", id, mk, formatNum(ma.Value), ma.Unit, "—", c.wrap(ansiBlue, "ONLY IN A"))
			default:
				deltaPct, verdict := compareMetric(mk, ma.Value, mb.Value, threshold, c)
				switch verdict.kind {
				case verdictRegression:
					regressions++
				case verdictImprovement:
					improvements++
				default:
					neutrals++
				}
				delta := mb.Value - ma.Value
				fmt.Fprintf(tw, "  %s\t%s\t%s %s\t%s %s\t%s%s\t%s\t%s\n",
					id, mk,
					formatNum(ma.Value), ma.Unit,
					formatNum(mb.Value), mb.Unit,
					signStr(delta), formatNum(math.Abs(delta)),
					formatPctSigned(deltaPct),
					verdict.text,
				)
			}
		}
	}
	_ = tw.Flush()

	// Summary.
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %s regressions, %s improvements, %s unchanged\n",
		c.wrap(ansiRed, fmt.Sprintf("%d", regressions)),
		c.wrap(ansiGreen, fmt.Sprintf("%d", improvements)),
		c.wrap(ansiGrey, fmt.Sprintf("%d", neutrals)),
	)
	return nil
}

func indexResults(results []runresult.Result) map[string]runresult.Result {
	out := make(map[string]runresult.Result, len(results))
	for _, r := range results {
		out[r.TaskID] = r
	}
	return out
}

func unionKeys(a, b map[string]runresult.Result) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func unionMetricKeys(a, b map[string]runresult.Metric) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

const (
	verdictNeutral     = iota // within threshold
	verdictRegression         // got worse
	verdictImprovement        // got better
)

type verdict struct {
	kind int
	text string
}

func compareMetric(key string, a, b, threshold float64, c tint) (deltaPct float64, v verdict) {
	if a == 0 && b == 0 {
		return 0, verdict{kind: verdictNeutral, text: c.wrap(ansiGrey, "≈")}
	}
	if a == 0 {
		// Going from 0 to something: large change, direction-aware.
		if lowerIsBetter(key) {
			return math.Inf(1), verdict{kind: verdictRegression, text: c.wrap(ansiRed, "↓ REGRESSION")}
		}
		return math.Inf(1), verdict{kind: verdictImprovement, text: c.wrap(ansiGreen, "↑ IMPROVEMENT")}
	}
	deltaPct = (b - a) / math.Abs(a) * 100
	if math.Abs(deltaPct) < threshold {
		return deltaPct, verdict{kind: verdictNeutral, text: c.wrap(ansiGrey, "≈")}
	}
	better := deltaPct > 0
	if lowerIsBetter(key) {
		better = !better
	}
	if better {
		return deltaPct, verdict{kind: verdictImprovement, text: c.wrap(ansiGreen, "↑ IMPROVEMENT")}
	}
	return deltaPct, verdict{kind: verdictRegression, text: c.wrap(ansiRed, "↓ REGRESSION")}
}

// lowerIsBetter applies a heuristic on the metric name.
func lowerIsBetter(key string) bool {
	k := strings.ToLower(key)
	for _, tag := range []string{"latency", "error_count", "duration_sec", "startup_p"} {
		if strings.Contains(k, tag) {
			return true
		}
	}
	return false
}

func statusVerdict(c tint, a, b runresult.Status) string {
	if a == b {
		return c.wrap(ansiGrey, "≈ same status")
	}
	if a != runresult.StatusOK && b == runresult.StatusOK {
		return c.wrap(ansiGreen, "↑ fixed")
	}
	if a == runresult.StatusOK && b != runresult.StatusOK {
		return c.wrap(ansiRed, "↓ broke")
	}
	return c.wrap(ansiYellow, fmt.Sprintf("%s → %s", a, b))
}

func formatNum(v float64) string {
	if v == 0 {
		return "0"
	}
	abs := math.Abs(v)
	switch {
	case abs >= 100:
		return fmt.Sprintf("%.0f", v)
	case abs >= 10:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}

func formatPctSigned(p float64) string {
	if math.IsInf(p, 1) {
		return "+∞%"
	}
	if math.IsInf(p, -1) {
		return "-∞%"
	}
	sign := "+"
	if p < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s%.1f%%", sign, math.Abs(p))
}

func signStr(v float64) string {
	if v >= 0 {
		return "+"
	}
	return "-"
}
