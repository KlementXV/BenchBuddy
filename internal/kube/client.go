package kube

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Options control how the kube client is constructed.
type Options struct {
	// KubeconfigPath overrides the kubeconfig location. If empty, the standard
	// resolution order is used: $KUBECONFIG, then $HOME/.kube/config.
	KubeconfigPath string
	// Context selects a specific context within the kubeconfig. Empty means
	// use the current-context.
	Context string
}

// Client wraps a kubernetes.Interface with the resolved config metadata.
type Client struct {
	clientset kubernetes.Interface
	context   string
	host      string
}

func (c *Client) Clientset() kubernetes.Interface { return c.clientset }
func (c *Client) Context() string                 { return c.context }
func (c *Client) Host() string                    { return c.host }

// NewClient resolves the kubeconfig and builds a typed clientset.
func NewClient(opts Options) (*Client, error) {
	loadRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.KubeconfigPath != "" {
		loadRules.ExplicitPath = opts.KubeconfigPath
	} else if env := os.Getenv("KUBECONFIG"); env != "" {
		loadRules.ExplicitPath = env
	} else if home, err := os.UserHomeDir(); err == nil {
		def := filepath.Join(home, ".kube", "config")
		if _, statErr := os.Stat(def); statErr == nil {
			loadRules.ExplicitPath = def
		}
	}

	if loadRules.ExplicitPath != "" {
		if _, err := os.Stat(loadRules.ExplicitPath); err != nil {
			return nil, fmt.Errorf("kubeconfig: %w", err)
		}
	}

	overrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		overrides.CurrentContext = opts.Context
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}
	if cfg.Host == "" {
		return nil, errors.New("kubeconfig: no host configured")
	}

	// The default client-go rate limiter (5 QPS / 10 burst) is too low for
	// BenchBuddy, which fans out many parallel create/get/watch calls across
	// dozens of tasks. Bump it so tasks aren't starved waiting on the limiter.
	cfg.QPS = 50
	cfg.Burst = 100

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}

	rawCfg, _ := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadRules, overrides).RawConfig()
	ctxName := overrides.CurrentContext
	if ctxName == "" {
		ctxName = rawCfg.CurrentContext
	}

	return &Client{clientset: cs, context: ctxName, host: cfg.Host}, nil
}

// NewFakeClient builds a Client backed by a provided fake clientset.
// Intended for use in tests of code that calls bench.Run().
func NewFakeClient(cs kubernetes.Interface) *Client {
	return &Client{clientset: cs, context: "fake", host: "fake://"}
}
