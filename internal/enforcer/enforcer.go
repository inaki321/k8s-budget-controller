package enforcer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

const originalReplicasAnnotation = "finops.io/original-replicas"

type Enforcer struct {
	kube      kubernetes.Interface
	metrics   metricsv.Interface
	namespace string
	selector  string
	dryRun    bool
}

func New(kube kubernetes.Interface, metrics metricsv.Interface, namespace, selector string, dryRun bool) *Enforcer {
	return &Enforcer{kube: kube, metrics: metrics, namespace: namespace, selector: selector, dryRun: dryRun}
}

// Usage returns the total CPU (cores) and memory (GiB) currently used by matching pods.
func (e *Enforcer) Usage(ctx context.Context) (cpuCores, memGiB float64, err error) {
	pods, err := e.metrics.MetricsV1beta1().PodMetricses(e.namespace).List(ctx, metav1.ListOptions{LabelSelector: e.selector})
	if err != nil {
		return 0, 0, fmt.Errorf("list pod metrics: %w", err)
	}
	for _, p := range pods.Items {
		for _, c := range p.Containers {
			cpuCores += float64(c.Usage.Cpu().MilliValue()) / 1000
			memGiB += float64(c.Usage.Memory().Value()) / (1 << 30)
		}
	}
	return cpuCores, memGiB, nil
}

// ScaleToZero scales matching deployments to 0, remembering the original replica count.
func (e *Enforcer) ScaleToZero(ctx context.Context) error {
	deps, err := e.kube.AppsV1().Deployments(e.namespace).List(ctx, metav1.ListOptions{LabelSelector: e.selector})
	if err != nil {
		return fmt.Errorf("list deployments: %w", err)
	}
	for _, d := range deps.Items {
		if d.Spec.Replicas == nil || *d.Spec.Replicas == 0 {
			continue
		}
		slog.Warn("scaling to zero", "deployment", d.Name, "from", *d.Spec.Replicas, "dryRun", e.dryRun)
		if e.dryRun {
			continue
		}
		patch := map[string]any{
			"metadata": map[string]any{"annotations": map[string]any{originalReplicasAnnotation: strconv.Itoa(int(*d.Spec.Replicas))}},
			"spec":     map[string]any{"replicas": 0},
		}
		if err := e.patch(ctx, d.Name, patch); err != nil {
			return err
		}
	}
	return nil
}

// Restore scales deployments we previously scaled down back to their original replicas.
func (e *Enforcer) Restore(ctx context.Context) error {
	deps, err := e.kube.AppsV1().Deployments(e.namespace).List(ctx, metav1.ListOptions{LabelSelector: e.selector})
	if err != nil {
		return fmt.Errorf("list deployments: %w", err)
	}
	for _, d := range deps.Items {
		v, ok := d.Annotations[originalReplicasAnnotation]
		if !ok {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("deployment %s: bad %s: %w", d.Name, originalReplicasAnnotation, err)
		}
		slog.Info("restoring", "deployment", d.Name, "replicas", n, "dryRun", e.dryRun)
		if e.dryRun {
			continue
		}
		patch := map[string]any{
			"metadata": map[string]any{"annotations": map[string]any{originalReplicasAnnotation: nil}},
			"spec":     map[string]any{"replicas": n},
		}
		if err := e.patch(ctx, d.Name, patch); err != nil {
			return err
		}
	}
	return nil
}

func (e *Enforcer) patch(ctx context.Context, name string, patch map[string]any) error {
	b, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = e.kube.AppsV1().Deployments(e.namespace).Patch(ctx, name, types.MergePatchType, b, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("patch deployment %s: %w", name, err)
	}
	return nil
}
