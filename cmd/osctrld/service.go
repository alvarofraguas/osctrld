package main

import (
	"context"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v2"
)

func intervalWithJitter(base time.Duration) time.Duration {
	jitterRange := float64(base) * 0.2
	jitter := rand.Float64()*jitterRange - jitterRange/2
	return base + time.Duration(jitter)
}

func syncOnce(c *cli.Context) {
	originalForce := jsonConfig.Force
	jsonConfig.Force = true
	defer func() { jsonConfig.Force = originalForce }()

	log.Info().Msg("syncing flags")
	if err := getFlags(c); err != nil {
		log.Error().Err(err).Msg("failed to sync flags")
	}
	log.Info().Msg("syncing cert")
	if err := getCert(c); err != nil {
		log.Error().Err(err).Msg("failed to sync cert")
	}
}

func serviceNode(c *cli.Context) error {
	interval := time.Duration(jsonConfig.Interval) * time.Minute
	log.Info().Int("interval_minutes", jsonConfig.Interval).Msg("starting service")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	syncOnce(c)

	for {
		wait := intervalWithJitter(interval)
		log.Debug().Dur("next_sync", wait).Msg("waiting for next sync")

		select {
		case <-time.After(wait):
			syncOnce(c)
		case <-ctx.Done():
			log.Info().Msg("shutting down")
			return nil
		}
	}
}
