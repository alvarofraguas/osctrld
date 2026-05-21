package main

import (
	"math/rand/v2"
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
	return nil
}
