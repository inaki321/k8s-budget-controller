package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Namespace           string
	Selector            string
	PollInterval        time.Duration
	BudgetWindow        time.Duration
	SpendLimit          float64 // USD per BudgetWindow
	CPUPricePerCoreHour float64
	MemPricePerGiBHour  float64
	DryRun              bool
}

func Load() (Config, error) {
	var err error
	c := Config{
		Namespace: os.Getenv("TARGET_NAMESPACE"),
		Selector:  os.Getenv("TARGET_SELECTOR"),
	}
	if c.Namespace == "" {
		return c, fmt.Errorf("TARGET_NAMESPACE is required")
	}
	if c.PollInterval, err = duration("POLL_INTERVAL", 30*time.Second); err != nil {
		return c, err
	}
	if c.BudgetWindow, err = duration("BUDGET_WINDOW", 168*time.Hour); err != nil {
		return c, err
	}
	if c.SpendLimit, err = float("SPEND_LIMIT", 0); err != nil {
		return c, err
	}
	if c.SpendLimit <= 0 {
		return c, fmt.Errorf("SPEND_LIMIT must be > 0")
	}
	if c.CPUPricePerCoreHour, err = float("CPU_PRICE_PER_CORE_HOUR", 0.031); err != nil {
		return c, err
	}
	if c.MemPricePerGiBHour, err = float("MEM_PRICE_PER_GIB_HOUR", 0.004); err != nil {
		return c, err
	}
	c.DryRun = os.Getenv("DRY_RUN") == "true"
	return c, nil
}

func duration(key string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

func float(key string, def float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return f, nil
}
