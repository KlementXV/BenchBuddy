package storage

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/clementlevoux/benchbuddy/internal/config"
	"github.com/clementlevoux/benchbuddy/internal/labels"
)

// PVC builds a PersistentVolumeClaim manifest for the storage bench.
func PVC(runID, namespace, storageClass, size string) *corev1.PersistentVolumeClaim {
	qty := resource.MustParse(size)
	scName := storageClass
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("bb-storage-%s-%s", sanitize(storageClass), shortID(runID)),
			Namespace: namespace,
			Labels:    labels.ForTask(runID, "storage", "storage/"+storageClass),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: qty,
				},
			},
		},
	}
}

// FioPod builds a Pod manifest that runs fio against the given PVC at /data.
func FioPod(runID, namespace, pvcName string, spec StorageTaskSpec, cfg config.RunConfig) *corev1.Pod {
	name := fmt.Sprintf("bb-storage-%s-%s-%s", sanitize(spec.Pattern), sanitize(spec.BlockSize), shortID(runID))
	lbls := labels.ForTask(runID, "storage", "storage/"+spec.StorageClass+"/"+spec.Pattern+"/"+spec.BlockSize)

	args := []string{
		"--bench=storage",
		"--directory=/data",
		"--pattern=" + spec.Pattern,
		"--block-size=" + spec.BlockSize,
		fmt.Sprintf("--duration=%s", cfg.Benches.Storage.Duration),
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    lbls,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			NodeSelector:  map[string]string{"kubernetes.io/hostname": spec.Node},
			Volumes: []corev1.Volume{{
				Name: "data",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvcName,
					},
				},
			}},
			Containers: []corev1.Container{{
				Name:            "runner",
				Image:           cfg.Images.Runner.FullRef(cfg.Images.Registry),
				ImagePullPolicy: corev1.PullPolicy(cfg.Images.Runner.PullPolicy),
				Args:            args,
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "data",
					MountPath: "/data",
				}},
			}},
			ImagePullSecrets: pullSecrets(cfg.Images.PullSecrets),
		},
	}
}

func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' {
			out = append(out, c)
		} else {
			out = append(out, '-')
		}
	}
	if len(out) > 24 {
		out = out[:24]
	}
	return string(out)
}

func pullSecrets(names []string) []corev1.LocalObjectReference {
	out := make([]corev1.LocalObjectReference, 0, len(names))
	for _, n := range names {
		out = append(out, corev1.LocalObjectReference{Name: n})
	}
	return out
}

func shortID(runID string) string {
	if len(runID) <= 8 {
		return runID
	}
	return runID[:8]
}
