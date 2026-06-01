package storage

import (
	"testing"

	"github.com/clementlevoux/benchbuddy/internal/config"
)

func TestPVCSpec(t *testing.T) {
	pvc := PVC("run-1", "ns", "gp3", "randread", "4k", "1Gi")
	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "gp3" {
		t.Errorf("storage class name not set correctly: %v", pvc.Spec.StorageClassName)
	}
	if pvc.Labels["benchbuddy.io/run-id"] != "run-1" {
		t.Errorf("missing run-id label: %v", pvc.Labels)
	}
}

func TestPodSpec_MountsPVC(t *testing.T) {
	cfg := config.RunConfig{
		Images: config.ImageConfig{
			Registry: "reg",
			Runner:   config.RunnerImage{Repository: "runner", Tag: "v1", PullPolicy: "IfNotPresent"},
		},
	}
	pod := FioPod("run-1", "ns", "pvc-name", StorageTaskSpec{
		StorageClass: "gp3", Pattern: "randread", BlockSize: "4k",
	}, cfg)
	if len(pod.Spec.Volumes) != 1 || pod.Spec.Volumes[0].PersistentVolumeClaim == nil {
		t.Fatalf("expected PVC volume, got %+v", pod.Spec.Volumes)
	}
	if pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName != "pvc-name" {
		t.Errorf("claim name: %v", pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName)
	}
}
