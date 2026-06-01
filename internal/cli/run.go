package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/clementlevoux/benchbuddy/internal/advisor"
	"github.com/clementlevoux/benchbuddy/internal/benches/api"
	"github.com/clementlevoux/benchbuddy/internal/benches/dns"
	"github.com/clementlevoux/benchbuddy/internal/benches/network"
	"github.com/clementlevoux/benchbuddy/internal/benches/pod"
	"github.com/clementlevoux/benchbuddy/internal/benches/storage"
	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/discover"
	"github.com/clementlevoux/benchbuddy/internal/kube"
	"github.com/clementlevoux/benchbuddy/internal/orchestrator"
	reportjson "github.com/clementlevoux/benchbuddy/internal/report/json"
	reportmd "github.com/clementlevoux/benchbuddy/internal/report/markdown"
	"github.com/clementlevoux/benchbuddy/internal/report/terminal"
	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

type runFlags struct {
	kubeconfig     string
	contextName    string
	configFile     string
	namespace      string
	profile        string
	parallelism    int
	parallelismSet bool
	timeout        time.Duration
	timeoutSet     bool
	keep           bool
	yes            bool

	excludeStorageClass []string
	excludeNode         []string
	excludeBench        []string
	includeStorageClass []string

	registry        string
	runnerImage     string
	runnerDigest    string
	imagePullSecret []string
	imagePullPolicy string

	output []string
}

func newRunCommand() *cobra.Command {
	var f runFlags
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run benchmarks against a Kubernetes cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f.parallelismSet = cmd.Flags().Changed("parallelism")
			f.timeoutSet = cmd.Flags().Changed("timeout")
			return runCmd(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), f)
		},
	}
	cmd.Flags().StringVar(&f.kubeconfig, "kubeconfig", "", "path to kubeconfig (default: $KUBECONFIG or ~/.kube/config)")
	cmd.Flags().StringVar(&f.contextName, "context", "", "kubeconfig context to use")
	cmd.Flags().StringVar(&f.configFile, "config", "", "path to benchbuddy YAML config file")
	cmd.Flags().StringVarP(&f.namespace, "namespace", "n", "", "target namespace (must already exist)")
	cmd.Flags().StringVar(&f.profile, "profile", "quick", "execution profile: quick | standard | deep")
	cmd.Flags().IntVar(&f.parallelism, "parallelism", 3, "max concurrent bench pods")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 0, "total run timeout (0 = use profile default)")
	cmd.Flags().BoolVar(&f.keep, "keep", false, "do not clean up resources after run")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip the preview confirmation prompt")
	cmd.Flags().StringSliceVar(&f.excludeStorageClass, "exclude-storageclass", nil, "storageClass names to skip")
	cmd.Flags().StringSliceVar(&f.excludeNode, "exclude-node", nil, "node names to skip")
	cmd.Flags().StringSliceVar(&f.excludeBench, "exclude-bench", nil, "bench names to skip")
	cmd.Flags().StringSliceVar(&f.includeStorageClass, "include-storageclass", nil, "storageClass names to test (whitelist; if empty, all are tested)")
	cmd.Flags().StringVar(&f.registry, "registry", "", "image registry base (overrides config)")
	cmd.Flags().StringVar(&f.runnerImage, "runner-image", "", "runner image as repository:tag (overrides config)")
	cmd.Flags().StringVar(&f.runnerDigest, "runner-digest", "", "runner image sha256 digest (overrides config)")
	cmd.Flags().StringSliceVar(&f.imagePullSecret, "image-pull-secret", nil, "imagePullSecrets in target namespace")
	cmd.Flags().StringVar(&f.imagePullPolicy, "image-pull-policy", "", "image pull policy (default IfNotPresent)")
	cmd.Flags().StringArrayVar(&f.output, "output", nil, "extra output: json=path.json or md=report.md (repeatable)")
	return cmd
}

func runCmd(parent context.Context, stdout, stderr io.Writer, f runFlags) error {
	if f.namespace == "" {
		return errors.New("--namespace is required (target namespace must already exist)")
	}

	jsonPath, mdPath, err := parseOutputs(f.output)
	if err != nil {
		return err
	}

	// 1. Connect.
	kc, err := kube.NewClient(kube.Options{KubeconfigPath: f.kubeconfig, Context: f.contextName})
	if err != nil {
		return fmt.Errorf("kube: %w", err)
	}
	fmt.Fprintf(stdout, "→ context: %s (%s)\n", kc.Context(), kc.Host())

	// 2. Build config.
	overrides := config.FlagOverrides{
		Namespace:    f.namespace,
		Profile:      f.profile,
		Keep:         f.keep,
		Yes:          f.yes,
		Excludes:     config.ExcludeConfig{StorageClasses: f.excludeStorageClass, Nodes: f.excludeNode, Benches: f.excludeBench},
		Includes:     config.IncludeConfig{StorageClasses: f.includeStorageClass},
		Registry:     f.registry,
		PullSecrets:  f.imagePullSecret,
		PullPolicy:   f.imagePullPolicy,
		RunnerDigest: f.runnerDigest,
	}
	if f.parallelismSet {
		overrides.Parallelism = &f.parallelism
	}
	if f.timeoutSet {
		overrides.Timeout = &f.timeout
	}
	if f.runnerImage != "" {
		repo, tag := splitImage(f.runnerImage)
		overrides.RunnerImageRepo, overrides.RunnerImageTag = repo, tag
	}
	cfg, err := config.Merge(config.MergeInputs{ProfileName: f.profile, Flags: overrides})
	if err != nil {
		return err
	}

	// 3. Preflight.
	ctx, cancel := context.WithTimeout(parent, maxDuration(cfg.Timeout, 5*time.Minute))
	defer cancel()
	if err := kube.CheckNamespace(ctx, kc.Clientset(), cfg.Namespace); err != nil {
		return err
	}
	missing, err := kube.CheckRBAC(ctx, kc.Clientset(), cfg.Namespace, kube.DefaultRBACRequirements())
	if err != nil {
		return fmt.Errorf("rbac check: %w", err)
	}
	if len(missing) > 0 {
		fmt.Fprintln(stderr, "✗ Missing RBAC permissions:")
		for _, m := range missing {
			fmt.Fprintf(stderr, "  - %s %s\n", m.Verb, m.Resource)
		}
		return errors.New("insufficient RBAC")
	}
	nodes, err := kube.CountUsableNodes(ctx, kc.Clientset())
	if err != nil {
		return err
	}
	if nodes < 2 {
		fmt.Fprintf(stderr, "warning: only %d usable node(s); cross-node bench will be skipped\n", nodes)
	}

	// 4. Discover.
	d, err := discover.Run(ctx, kc.Clientset())
	if err != nil {
		return fmt.Errorf("discover: %w", err)
	}

	// 5. Orchestrator + plan.
	runID := newRunID()
	o := orchestrator.New(kc, cfg)
	baseReader := podLogReader(kc)
	o.Register(network.New().WithLogReader(network.LogReader(baseReader)))
	o.Register(api.New())
	o.Register(dns.New().WithLogReader(dns.LogReader(baseReader)))
	o.Register(storage.New().WithLogReader(storage.LogReader(baseReader)))
	o.Register(pod.New().WithLogReader(pod.LogReader(baseReader)))
	plan, err := o.Plan(ctx, d)
	if err != nil {
		return err
	}

	// 6. Preview + confirm.
	fmt.Fprintf(stdout, "\nPlanned tasks (%d):\n", len(plan))
	for _, pt := range plan {
		fmt.Fprintf(stdout, "  • %s — %s\n", pt.Task.ID, pt.Task.Subject)
	}
	fmt.Fprintln(stdout)
	if !cfg.Yes {
		if !confirm("Proceed with this plan? [y/N]: ") {
			fmt.Fprintln(stdout, "aborted")
			return nil
		}
	}

	// 7. Run (with signal trap).
	runCtx, drain := orchestrator.TrapSignals(ctx)
	defer drain()
	start := time.Now()
	results, runErr := o.Run(runCtx, runID, plan)

	// 8. Cleanup (unless --keep).
	if !cfg.Keep {
		ccx, ccancel := orchestrator.CleanupContext(60 * time.Second)
		_ = orchestrator.CleanupByLabel(ccx, kc.Clientset(), cfg.Namespace, runID)
		ccancel()
	}

	if runErr != nil {
		return runErr
	}

	// 9. Advise.
	report := runresult.Report{
		Meta: runresult.RunMeta{
			RunID:      runID,
			Profile:    cfg.Profile,
			Namespace:  cfg.Namespace,
			K8sVersion: d.K8sVersion,
			CNI:        d.CNI,
			NodeCount:  len(d.Nodes),
			StartedAt:  start,
			Duration:   time.Since(start),
		},
		Results:  results,
		Findings: advisor.Evaluate(results),
	}

	// 10. Render terminal (always).
	if err := terminal.Render(stdout, report, terminal.RenderOptions{}); err != nil {
		return err
	}

	// 11. Render additional output formats.
	if jsonPath != "" {
		f, err := os.Create(jsonPath)
		if err != nil {
			return fmt.Errorf("create json output %s: %w", jsonPath, err)
		}
		defer f.Close()
		if err := reportjson.Render(f, report); err != nil {
			return err
		}
	}
	if mdPath != "" {
		f, err := os.Create(mdPath)
		if err != nil {
			return fmt.Errorf("create markdown output %s: %w", mdPath, err)
		}
		defer f.Close()
		if err := reportmd.Render(f, report); err != nil {
			return err
		}
	}

	// 12. Exit code 1 if any HIGH or CRITICAL finding.
	for _, finding := range report.Findings {
		if finding.Severity == runresult.SeverityHigh || finding.Severity == runresult.SeverityCritical {
			return ErrHighFindings
		}
	}
	return nil
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	var line string
	_, _ = fmt.Fscanln(os.Stdin, &line)
	return line == "y" || line == "Y" || line == "yes"
}

func newRunID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func splitImage(s string) (repo, tag string) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func parseOutputs(flags []string) (jsonPath, mdPath string, err error) {
	for _, o := range flags {
		idx := strings.Index(o, "=")
		if idx < 0 {
			return "", "", fmt.Errorf("invalid --output %q: expected format=path (e.g. json=out.json)", o)
		}
		format, path := o[:idx], o[idx+1:]
		switch format {
		case "json":
			jsonPath = path
		case "md":
			mdPath = path
		default:
			return "", "", fmt.Errorf("unsupported output format %q: use json or md", format)
		}
	}
	return
}

// podLogReader returns a LogReader backed by the real clientset.
func podLogReader(kc *kube.Client) network.LogReader {
	return func(ctx context.Context, namespace, podName string) ([]byte, error) {
		req := kc.Clientset().CoreV1().Pods(namespace).GetLogs(podName, nil)
		rc, err := req.Stream(ctx)
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		buf := make([]byte, 0, 64*1024)
		for {
			tmp := make([]byte, 8192)
			n, rerr := rc.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if rerr != nil {
				break
			}
		}
		return buf, nil
	}
}
