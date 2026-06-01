package advisor

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/runresult"
)

func storageResult(taskID string, iops, bwMBs, p99us float64) runresult.Result {
	return runresult.Result{
		BenchName: "storage",
		TaskID:    taskID,
		Status:    runresult.StatusOK,
		Metrics: map[string]runresult.Metric{
			"iops":           {Value: iops, Unit: "iops"},
			"bandwidth_mb_s": {Value: bwMBs, Unit: "MB/s"},
			"latency_p99_us": {Value: p99us, Unit: "us"},
		},
	}
}

func TestRuleStorageRandReadIOPSLow(t *testing.T) {
	findings := ruleStorageRandReadIOPSLow.Evaluate([]runresult.Result{
		storageResult("storage/standard/randread/4k", 500, 50, 5000),
	})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM finding, got %v", findings)
	}
	findings = ruleStorageRandReadIOPSLow.Evaluate([]runresult.Result{
		storageResult("storage/standard/randread/4k", 2000, 50, 5000),
	})
	if len(findings) != 0 {
		t.Error("expected 0 findings above threshold")
	}
	// non-randread task should not trigger
	findings = ruleStorageRandReadIOPSLow.Evaluate([]runresult.Result{
		storageResult("storage/standard/seqread/4k", 500, 50, 5000),
	})
	if len(findings) != 0 {
		t.Error("seqread task should not trigger randread rule")
	}
}

func TestRuleStorageRandWriteIOPSLow(t *testing.T) {
	findings := ruleStorageRandWriteIOPSLow.Evaluate([]runresult.Result{
		storageResult("storage/standard/randwrite/4k", 200, 20, 5000),
	})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityHigh {
		t.Errorf("expected 1 HIGH finding, got %v", findings)
	}
	findings = ruleStorageRandWriteIOPSLow.Evaluate([]runresult.Result{
		storageResult("storage/standard/randwrite/4k", 800, 80, 5000),
	})
	if len(findings) != 0 {
		t.Error("expected 0 findings above threshold")
	}
}

func TestRuleStorageReadLatencyHigh(t *testing.T) {
	// 15000 us = 15 ms > 10 ms threshold
	findings := ruleStorageReadLatencyHigh.Evaluate([]runresult.Result{
		storageResult("storage/standard/randread/4k", 500, 50, 15000),
	})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityMedium {
		t.Errorf("expected 1 MEDIUM finding, got %v", findings)
	}
	findings = ruleStorageReadLatencyHigh.Evaluate([]runresult.Result{
		storageResult("storage/standard/randread/4k", 500, 50, 5000),
	})
	if len(findings) != 0 {
		t.Error("expected 0 findings below threshold")
	}
}

func TestRuleStorageWriteLatencyHigh(t *testing.T) {
	// 25000 us = 25 ms > 20 ms threshold
	findings := ruleStorageWriteLatencyHigh.Evaluate([]runresult.Result{
		storageResult("storage/standard/randwrite/4k", 500, 50, 25000),
	})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityHigh {
		t.Errorf("expected 1 HIGH finding, got %v", findings)
	}
}

func TestRuleStorageSeqReadBandwidthLow(t *testing.T) {
	findings := ruleStorageSeqReadBandwidthLow.Evaluate([]runresult.Result{
		storageResult("storage/standard/seqread/1M", 100, 50, 5000),
	})
	if len(findings) != 1 || findings[0].Severity != runresult.SeverityLow {
		t.Errorf("expected 1 LOW finding, got %v", findings)
	}
	findings = ruleStorageSeqReadBandwidthLow.Evaluate([]runresult.Result{
		storageResult("storage/standard/seqread/1M", 100, 200, 5000),
	})
	if len(findings) != 0 {
		t.Error("expected 0 findings above threshold")
	}
}
