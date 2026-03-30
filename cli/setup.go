package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	return runInteractiveSetup(os.Stdin, os.Stdout)
}

func runInteractiveSetup(in io.Reader, out io.Writer) error {
	configPath, err := internal.DefaultConfigPath()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(in)
	fmt.Fprintln(out, "Daily Tasks needs a backend before you can continue.")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Choose a backend:")
	fmt.Fprintln(out, "  1. Local only")
	fmt.Fprintln(out, "  2. Nextcloud")
	fmt.Fprint(out, "Enter 1 or 2: ")

	choice, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1", "local":
		if err := internal.SaveAppConfig(configPath, internal.AppConfig{Backend: internal.BackendLocal}); err != nil {
			return err
		}
		fmt.Fprintln(out, "Saved local-only backend configuration.")
		return nil
	case "2", "nextcloud":
		fmt.Fprint(out, "Nextcloud server URL: ")
		serverURL, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		serverURL = strings.TrimSpace(serverURL)
		if serverURL == "" {
			return errors.New("server URL is required")
		}

		session, err := internal.StartLoginFlowV2(serverURL)
		if err != nil {
			return err
		}

		openBrowser(session.LoginURL)
		fmt.Fprintln(out, "Opened Nextcloud login in your browser.")
		fmt.Fprintf(out, "If the browser did not open, use this URL:\n%s\n", session.LoginURL)

		fmt.Fprint(out, "Waiting for Nextcloud authorization")
		deadline := time.Now().Add(20 * time.Minute)
		for time.Now().Before(deadline) {
			result, complete, err := internal.PollLoginFlowV2(session)
			if err != nil {
				return err
			}
			if complete {
				if err := internal.SaveAppConfig(configPath, internal.AppConfigFromLogin(result)); err != nil {
					return err
				}
				fmt.Fprintln(out, "")
				fmt.Fprintf(out, "Connected to Nextcloud as %s.\n", result.LoginName)
				return nil
			}
			fmt.Fprint(out, ".")
			time.Sleep(2 * time.Second)
		}

		fmt.Fprintln(out, "")
		return errors.New("timed out waiting for Nextcloud authorization")
	default:
		return errors.New("choose either 1 or 2")
	}
}
