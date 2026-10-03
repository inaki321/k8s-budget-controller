package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/inaki00/finops/internal/config"
	"github.com/inaki00/finops/internal/cost"
	"github.com/inaki00/finops/internal/enforcer"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	rc, err := restConfig()
	if err != nil {
		slog.Error("kube config", "err", err)
		os.Exit(1)
	}
	kube := kubernetes.NewForConfigOrDie(rc)
	metrics := metricsv.NewForConfigOrDie(rc)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	enf := enforcer.New(kube, metrics, cfg.Namespace, cfg.Selector, cfg.DryRun)
	tracker := cost.NewTracker(time.Now(), cfg.BudgetWindow, cfg.CPUPricePerCoreHour, cfg.MemPricePerGiBHour)
	enforced := false

	slog.Info("started", "namespace", cfg.Namespace, "selector", cfg.Selector, "limitUSD", cfg.SpendLimit,
		"poll", cfg.PollInterval, "window", cfg.BudgetWindow, "dryRun", cfg.DryRun)

	t := time.NewTicker(cfg.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			cpu, mem, err := enf.Usage(ctx)
			if err != nil {
				slog.Error("usage", "err", err)
				continue
			}
			spend, reset := tracker.Observe(now, cpu, mem)
			slog.Info("tick", "cpuCores", cpu, "memGiB", mem, "spendUSD", spend, "limitUSD", cfg.SpendLimit, "enforced", enforced)

			switch {
			case reset && enforced:
				if err := enf.Restore(ctx); err != nil {
					slog.Error("restore", "err", err)
					continue
				}
				enforced = false
			case !enforced && spend >= cfg.SpendLimit:
				if err := enf.ScaleToZero(ctx); err != nil {
					slog.Error("scale to zero", "err", err)
					continue
				}
				enforced = true
			}
		}
	}
}

func restConfig() (*rest.Config, error) {
	if rc, err := rest.InClusterConfig(); err == nil {
		return rc, nil
	}
	return clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
}
