package runner

import (
	"testing"
	"time"
)

func TestParseDNSArgs_Defaults(t *testing.T) {
	a, err := ParseDNSArgs([]string{"--queries=foo.bar.svc,baz.svc"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Duration != 10*time.Second {
		t.Errorf("default duration: %v", a.Duration)
	}
	if a.QPS != 50 {
		t.Errorf("default qps: %d", a.QPS)
	}
	if len(a.Queries) != 2 {
		t.Errorf("queries: %v", a.Queries)
	}
}

func TestParseDNSArgs_Custom(t *testing.T) {
	a, err := ParseDNSArgs([]string{"--queries=x.y", "--duration=30s", "--qps=200"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Duration != 30*time.Second || a.QPS != 200 {
		t.Errorf("got %+v", a)
	}
}

func TestParseDNSArgs_RequiresQueries(t *testing.T) {
	_, err := ParseDNSArgs([]string{"--duration=1s"})
	if err == nil {
		t.Fatal("expected error on missing --queries")
	}
}
