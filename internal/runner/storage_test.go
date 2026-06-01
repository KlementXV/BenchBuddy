package runner

import (
	"testing"
	"time"
)

func TestParseStorageArgs_Defaults(t *testing.T) {
	a, err := ParseStorageArgs([]string{"--directory=/data", "--pattern=randread", "--block-size=4k"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Directory != "/data" || a.Pattern != "randread" || a.BlockSize != "4k" {
		t.Errorf("got %+v", a)
	}
	if a.Duration != 15*time.Second {
		t.Errorf("default duration: %v", a.Duration)
	}
}

func TestParseStorageArgs_RequiresDirectory(t *testing.T) {
	_, err := ParseStorageArgs([]string{"--pattern=randread", "--block-size=4k"})
	if err == nil {
		t.Fatal("expected error on missing --directory")
	}
}

func TestParseStorageArgs_ValidatesPattern(t *testing.T) {
	_, err := ParseStorageArgs([]string{"--directory=/d", "--pattern=invalid", "--block-size=4k"})
	if err == nil {
		t.Fatal("expected error on invalid pattern")
	}
}
