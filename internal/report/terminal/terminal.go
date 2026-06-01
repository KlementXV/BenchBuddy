package terminal

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

// RenderOptions controls optional rendering behaviour.
type RenderOptions struct {
	NoColor bool
}

// Render writes a structured terminal report to w. NoColor=true is used in
// tests to keep golden files stable.
func Render(w io.Writer, r runresult.Report, opts RenderOptions) error {
	fmt.Fprintln(w, "BenchBuddy Report")
	fmt.Fprintf(w, "  run:        %s\n", r.Meta.RunID)
	fmt.Fprintf(w, "  profile:    %s\n", r.Meta.Profile)
	fmt.Fprintf(w, "  namespace:  %s\n", r.Meta.Namespace)
	fmt.Fprintf(w, "  cluster:    k8s=%s cni=%s nodes=%d\n", r.Meta.K8sVersion, r.Meta.CNI, r.Meta.NodeCount)
	fmt.Fprintf(w, "  duration:   %s\n", r.Meta.Duration)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Results:")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  BENCH\tSUBJECT\tSTATUS\tMETRICS")
	for _, res := range r.Results {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
			res.BenchName, res.Subject, res.Status, formatMetrics(res.Metrics))
	}
	_ = tw.Flush()

	if len(r.Findings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Findings:")
		for _, f := range r.Findings {
			fmt.Fprintf(w, "  [%s] %s/%s: %s\n", f.Severity, f.Category, f.Title, f.Detail)
			for _, fix := range f.Fix {
				fmt.Fprintf(w, "    → %s\n", fix)
			}
		}
	}
	return nil
}

func formatMetrics(m map[string]runresult.Metric) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += "  "
		}
		out += fmt.Sprintf("%s=%.2f%s", k, m[k].Value, unitSuffix(m[k].Unit))
	}
	return out
}

func unitSuffix(u string) string {
	if u == "" {
		return ""
	}
	return " " + u
}
