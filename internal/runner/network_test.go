package runner

import (
	"testing"
)

func TestParseNetworkArgs_Server(t *testing.T) {
	a, err := ParseNetworkArgs([]string{"--role=server", "--port=5201"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Role != "server" || a.Port != 5201 {
		t.Errorf("got %#v", a)
	}
}

func TestParseNetworkArgs_Client(t *testing.T) {
	a, err := ParseNetworkArgs([]string{
		"--role=client",
		"--target=10.0.0.5",
		"--protocol=udp",
		"--duration=10s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if a.Role != "client" || a.Target != "10.0.0.5" || a.Protocol != "udp" {
		t.Errorf("got %#v", a)
	}
}

func TestParseNetworkArgs_BadRole(t *testing.T) {
	_, err := ParseNetworkArgs([]string{"--role=banana"})
	if err == nil {
		t.Fatal("expected error")
	}
}
