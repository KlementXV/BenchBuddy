package advisor

import (
	"fmt"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func init() {
	RegisteredRules = append(RegisteredRules,
		ruleNetCrossNodeTCPDegradation,
		ruleNetCrossNodeTCPLow,
		ruleNetSameNodeTCPLow,
		ruleNetUDPPacketLoss,
		ruleNetUDPJitter,
	)
}

var ruleNetCrossNodeTCPDegradation = Rule{
	ID: "net-cross-node-tcp-degradation",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		same := metricsFor(results, "network/same-node/tcp")
		cross := metricsFor(results, "network/cross-node/tcp")
		if same == nil || cross == nil {
			return nil
		}
		sameGbps := metricVal(same, "bandwidth_sent_gbps")
		crossGbps := metricVal(cross, "bandwidth_sent_gbps")
		if sameGbps <= 0 || crossGbps >= 0.7*sameGbps {
			return nil
		}
		ratio := int(crossGbps / sameGbps * 100)
		return []runresult.Finding{{
			Severity: runresult.SeverityMedium,
			Category: "network",
			Title:    "Cross-node TCP bandwidth degradation",
			Detail:   fmt.Sprintf("Cross-node %.2f Gbps is %d%% of same-node %.2f Gbps. CNI overhead suspected.", crossGbps, ratio, sameGbps),
			Fix: []string{
				"Check CNI encapsulation mode (VXLAN/IPIP adds overhead on Calico, Flannel)",
				"Increase MTU to ≥ 9000 if your fabric supports jumbo frames",
				"Consider native-routing CNIs: Cilium native mode, Calico BGP",
			},
			Refs:      []string{"https://docs.cilium.io/en/stable/operations/performance/"},
			SourceIDs: []string{"network/same-node/tcp", "network/cross-node/tcp"},
		}}
	},
}

var ruleNetCrossNodeTCPLow = Rule{
	ID: "net-cross-node-tcp-low",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "network/cross-node/tcp")
		if m == nil {
			return nil
		}
		gbps := metricVal(m, "bandwidth_sent_gbps")
		if gbps <= 0 || gbps >= 1.0 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityHigh,
			Category: "network",
			Title:    "Cross-node TCP bandwidth critically low",
			Detail:   fmt.Sprintf("Cross-node TCP bandwidth %.2f Gbps is below 1 Gbps. Pod-to-pod traffic will be severely constrained.", gbps),
			Fix: []string{
				"Check physical NIC speed and duplex settings on both nodes",
				"Investigate CNI configuration and any network policy rate-limiting rules",
				"Check for noisy-neighbour activity saturating the node NIC queue",
			},
			SourceIDs: []string{"network/cross-node/tcp"},
		}}
	},
}

var ruleNetSameNodeTCPLow = Rule{
	ID: "net-same-node-tcp-low",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "network/same-node/tcp")
		if m == nil {
			return nil
		}
		gbps := metricVal(m, "bandwidth_sent_gbps")
		if gbps <= 0 || gbps >= 5.0 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityMedium,
			Category: "network",
			Title:    "Same-node TCP bandwidth low",
			Detail:   fmt.Sprintf("Same-node TCP bandwidth %.2f Gbps is below 5 Gbps. Local pod communication is slower than expected for kernel loopback.", gbps),
			Fix: []string{
				"Check if CNI applies iptables policy on loopback traffic (should be bypassed)",
				"Inspect CPU throttling affecting the kernel network stack on this node",
			},
			SourceIDs: []string{"network/same-node/tcp"},
		}}
	},
}

var ruleNetUDPPacketLoss = Rule{
	ID: "net-udp-packet-loss",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		sourceID := "network/cross-node/udp"
		m := metricsFor(results, sourceID)
		if m == nil {
			sourceID = "network/same-node/udp"
			m = metricsFor(results, sourceID)
		}
		if m == nil {
			return nil
		}
		lost := metricVal(m, "lost_percent")
		if lost < 1.0 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityMedium,
			Category: "network",
			Title:    "UDP packet loss detected",
			Detail:   fmt.Sprintf("UDP packet loss is %.1f%%. Real-time workloads (VoIP, game servers, QUIC) will be impacted.", lost),
			Fix: []string{
				"Check UDP receive buffer size: sysctl net.core.rmem_max",
				"Investigate NIC ring buffer drops: ethtool -S <iface> | grep drop",
				"Check for UDP rate limiting in network policy or iptables rules",
			},
			SourceIDs: []string{sourceID},
		}}
	},
}

var ruleNetUDPJitter = Rule{
	ID: "net-udp-jitter-high",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		m := metricsFor(results, "network/cross-node/udp")
		if m == nil {
			return nil
		}
		jitter := metricVal(m, "jitter_ms")
		if jitter <= 0 || jitter < 5.0 {
			return nil
		}
		return []runresult.Finding{{
			Severity: runresult.SeverityLow,
			Category: "network",
			Title:    "High UDP jitter",
			Detail:   fmt.Sprintf("UDP jitter is %.1f ms. Latency-sensitive workloads will experience inconsistent performance.", jitter),
			Fix: []string{
				"Enable QoS/traffic shaping to prioritize latency-sensitive flows",
				"Check for competing bandwidth-intensive workloads on the same nodes",
			},
			SourceIDs: []string{"network/cross-node/udp"},
		}}
	},
}
