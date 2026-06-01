package advisor

import (
	"fmt"
	"strings"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func init() {
	RegisteredRules = append(RegisteredRules,
		ruleStorageRandReadIOPSLow,
		ruleStorageRandWriteIOPSLow,
		ruleStorageReadLatencyHigh,
		ruleStorageWriteLatencyHigh,
		ruleStorageSeqReadBandwidthLow,
	)
}

var ruleStorageRandReadIOPSLow = Rule{
	ID: "storage-randread-iops-low",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		var findings []runresult.Finding
		for _, r := range allByBench(results, "storage") {
			if !strings.Contains(r.TaskID, "/randread/") {
				continue
			}
			iops := metricVal(r.Metrics, "iops")
			if iops <= 0 || iops >= 1000 {
				continue
			}
			findings = append(findings, runresult.Finding{
				Severity:  runresult.SeverityMedium,
				Category:  "storage",
				Title:     "Low random-read IOPS",
				Detail:    fmt.Sprintf("Task %s achieved %.0f random-read IOPS (threshold: 1000).", r.TaskID, iops),
				Fix: []string{
					"Verify the StorageClass uses SSD-backed volumes (not spinning disk)",
					"Check if the volume is thin-provisioned and needs pre-warming",
					"Consider a StorageClass with local volumes for IOPS-intensive workloads",
				},
				SourceIDs: []string{r.TaskID},
			})
		}
		return findings
	},
}

var ruleStorageRandWriteIOPSLow = Rule{
	ID: "storage-randwrite-iops-low",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		var findings []runresult.Finding
		for _, r := range allByBench(results, "storage") {
			if !strings.Contains(r.TaskID, "/randwrite/") {
				continue
			}
			iops := metricVal(r.Metrics, "iops")
			if iops <= 0 || iops >= 500 {
				continue
			}
			findings = append(findings, runresult.Finding{
				Severity:  runresult.SeverityHigh,
				Category:  "storage",
				Title:     "Low random-write IOPS",
				Detail:    fmt.Sprintf("Task %s achieved %.0f random-write IOPS (threshold: 500). Databases will be severely impacted.", r.TaskID, iops),
				Fix: []string{
					"Enable write-back caching on the storage backend if data durability allows",
					"Check if cloud I/O credit bucket is exhausted (AWS gp2, Azure Standard SSD)",
					"Upgrade to a higher-performance StorageClass (gp3, Premium SSD, io1/io2)",
				},
				SourceIDs: []string{r.TaskID},
			})
		}
		return findings
	},
}

var ruleStorageReadLatencyHigh = Rule{
	ID: "storage-read-latency-high",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		var findings []runresult.Finding
		for _, r := range allByBench(results, "storage") {
			if !strings.Contains(r.TaskID, "/randread/") && !strings.Contains(r.TaskID, "/seqread/") {
				continue
			}
			p99us := metricVal(r.Metrics, "latency_p99_us")
			if p99us <= 0 || p99us < 10000 {
				continue
			}
			findings = append(findings, runresult.Finding{
				Severity:  runresult.SeverityMedium,
				Category:  "storage",
				Title:     "High storage read latency",
				Detail:    fmt.Sprintf("Task %s: p99 read latency is %.1f ms (threshold: 10 ms).", r.TaskID, p99us/1000),
				Fix: []string{
					"Check storage network congestion (NFS/Ceph/iSCSI backends)",
					"Review StorageClass reclaim policy and garbage collection overhead",
					"Tune ReadAheadKB for sequential-read-heavy workloads",
				},
				SourceIDs: []string{r.TaskID},
			})
		}
		return findings
	},
}

var ruleStorageWriteLatencyHigh = Rule{
	ID: "storage-write-latency-high",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		var findings []runresult.Finding
		for _, r := range allByBench(results, "storage") {
			if !strings.Contains(r.TaskID, "/randwrite/") && !strings.Contains(r.TaskID, "/seqwrite/") {
				continue
			}
			p99us := metricVal(r.Metrics, "latency_p99_us")
			if p99us <= 0 || p99us < 20000 {
				continue
			}
			findings = append(findings, runresult.Finding{
				Severity:  runresult.SeverityHigh,
				Category:  "storage",
				Title:     "High storage write latency",
				Detail:    fmt.Sprintf("Task %s: p99 write latency is %.1f ms (threshold: 20 ms). Transactional workloads will be impacted.", r.TaskID, p99us/1000),
				Fix: []string{
					"Enable write caching if data loss on failure is acceptable",
					"Check storage backend replication synchrony (synchronous Ceph, SAN multi-path)",
					"Use local-path StorageClass for latency-sensitive stateful workloads",
				},
				SourceIDs: []string{r.TaskID},
			})
		}
		return findings
	},
}

var ruleStorageSeqReadBandwidthLow = Rule{
	ID: "storage-seqread-bandwidth-low",
	Evaluate: func(results []runresult.Result) []runresult.Finding {
		var findings []runresult.Finding
		for _, r := range allByBench(results, "storage") {
			if !strings.Contains(r.TaskID, "/seqread/") {
				continue
			}
			bw := metricVal(r.Metrics, "bandwidth_mb_s")
			if bw <= 0 || bw >= 100 {
				continue
			}
			findings = append(findings, runresult.Finding{
				Severity:  runresult.SeverityLow,
				Category:  "storage",
				Title:     "Low sequential-read bandwidth",
				Detail:    fmt.Sprintf("Task %s: sequential-read bandwidth is %.0f MB/s (threshold: 100 MB/s). Bulk data loads will be slow.", r.TaskID, bw),
				Fix: []string{
					"Check if storage backend network is saturated during the bench",
					"Tune ReadAheadKB and queue depth for sequential workloads",
					"Consider StorageClass with dedicated network bandwidth (provisioned IOPS)",
				},
				SourceIDs: []string{r.TaskID},
			})
		}
		return findings
	},
}
