package runresult

import (
	"errors"
	"strings"
	"testing"
)

func TestParseMarker(t *testing.T) {
	tests := []struct {
		name      string
		log       string
		wantOK    bool
		wantValue float64
	}{
		{
			name:      "single marker on its own line",
			log:       "doing work\nBENCHBUDDY_RESULT: {\"bandwidth_gbps\":{\"value\":9.1,\"unit\":\"Gbps\"}}\n",
			wantOK:    true,
			wantValue: 9.1,
		},
		{
			name:   "no marker present",
			log:    "iperf3 ran but did not emit\nstuff\n",
			wantOK: false,
		},
		{
			name:      "multiple markers — last one wins",
			log:       "BENCHBUDDY_RESULT: {\"bandwidth_gbps\":{\"value\":1,\"unit\":\"Gbps\"}}\nlater\nBENCHBUDDY_RESULT: {\"bandwidth_gbps\":{\"value\":9.1,\"unit\":\"Gbps\"}}\n",
			wantOK:    true,
			wantValue: 9.1,
		},
		{
			name:   "malformed JSON after marker",
			log:    "BENCHBUDDY_RESULT: {not json",
			wantOK: false,
		},
		{
			name:      "marker with leading whitespace on line",
			log:       "  BENCHBUDDY_RESULT: {\"bandwidth_gbps\":{\"value\":5,\"unit\":\"Gbps\"}}",
			wantOK:    true,
			wantValue: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metrics, _, err := ParseMarker(strings.NewReader(tc.log))
			if tc.wantOK {
				if err != nil {
					t.Fatalf("expected ok, got err: %v", err)
				}
				if metrics["bandwidth_gbps"].Value != tc.wantValue {
					t.Errorf("got value %v, want %v", metrics["bandwidth_gbps"].Value, tc.wantValue)
				}
			} else {
				if err == nil {
					t.Fatalf("expected error, got metrics: %#v", metrics)
				}
			}
		})
	}
}

func TestEmitMarker(t *testing.T) {
	var sb strings.Builder
	metrics := map[string]Metric{
		"iops_read": {Value: 12450, Unit: "iops"},
	}
	if err := EmitMarker(&sb, metrics); err != nil {
		t.Fatalf("EmitMarker error: %v", err)
	}
	got := sb.String()
	if !strings.HasPrefix(got, MarkerPrefix) {
		t.Errorf("output does not start with marker prefix, got: %q", got)
	}
	if !strings.Contains(got, "12450") {
		t.Errorf("output missing value, got: %q", got)
	}
}

type failingReader struct {
	bytesRead int
	failAfter int
}

func (f *failingReader) Read(p []byte) (int, error) {
	if f.bytesRead >= f.failAfter {
		return 0, errors.New("simulated read failure")
	}
	n := copy(p, []byte("BENCHBUDDY_RESULT: {"))
	f.bytesRead += n
	if f.bytesRead >= f.failAfter {
		return n, errors.New("simulated read failure")
	}
	return n, nil
}

func TestParseMarker_ScannerError(t *testing.T) {
	_, _, err := ParseMarker(&failingReader{failAfter: 5})
	if err == nil {
		t.Fatal("expected error from failing reader, got nil")
	}
}
