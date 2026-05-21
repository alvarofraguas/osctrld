package main

import (
	"math/rand/v2"
	"time"

	"github.com/urfave/cli/v2"
)

func intervalWithJitter(base time.Duration) time.Duration {
	jitterRange := float64(base) * 0.2
	jitter := rand.Float64()*jitterRange - jitterRange/2
	return base + time.Duration(jitter)
}

func serviceNode(c *cli.Context) error {
	return nil
}
