package main

import (
	"errors"
	"fmt"

	"daily-tasks/internal"
)

func requireConfiguredBackend() error {
	cfg, _, err := internal.LoadEffectiveAppConfig()
	if err != nil {
		return err
	}
	if internal.IsBackendConfigured(cfg) {
		return nil
	}
	return fmt.Errorf("%w. Run `daily-tasks setup` first", internal.ErrBackendNotConfigured)
}

func ensureConfiguredBackendInteractive() error {
	err := requireConfiguredBackend()
	if err == nil {
		return nil
	}
	if !errors.Is(err, internal.ErrBackendNotConfigured) {
		return err
	}
	if err := internal.RunSetupTUI(); err != nil {
		return err
	}
	return requireConfiguredBackend()
}
