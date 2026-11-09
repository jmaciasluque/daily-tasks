package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"daily-tasks/internal"
)

func printAccountUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: daily-tasks account [delete [--yes]]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Without arguments, shows the hosted account you are signed in with.")
	fmt.Fprintln(w, "  delete   Permanently delete the hosted account and all synced data")
}

// hostedBackendFromConfig returns the signed-in hosted backend and token path.
func hostedBackendFromConfig() (*internal.HostedBackend, string, error) {
	cfg, _, err := internal.LoadDefaultAppConfig()
	if err != nil {
		return nil, "", err
	}
	if cfg.Backend != internal.BackendHosted {
		return nil, "", errors.New("not using the hosted backend; run daily-tasks login first")
	}
	tokenPath, err := internal.DefaultHostedTokenPath()
	if err != nil {
		return nil, "", err
	}
	token, err := internal.LoadHostedToken(tokenPath)
	if err != nil || token == "" {
		return nil, "", internal.ErrHostedTokenMissing
	}
	return internal.NewHostedBackend(cfg.Hosted.APIURL, token), tokenPath, nil
}

func runAccount(args []string) error {
	if wantsHelp(args) {
		printAccountUsage(os.Stdout)
		return nil
	}
	if len(args) > 0 && args[0] == "delete" {
		return runAccountDelete(args[1:], os.Stdin, os.Stdout)
	}
	if len(args) > 0 {
		return fmt.Errorf("unknown account command: %s", args[0])
	}
	backend, _, err := hostedBackendFromConfig()
	if err != nil {
		return err
	}
	acct, err := backend.Account()
	if err != nil {
		return err
	}
	email := acct.Email
	if email == "" {
		email = "(no email)"
	}
	fmt.Printf("Signed in as %s via %s\n", email, acct.Provider)
	fmt.Printf("Server:        %s\n", backend.APIURL)
	fmt.Printf("Account since: %s\n", acct.CreatedAt.Local().Format("2006-01-02"))
	if !acct.ExpiresAt.IsZero() {
		fmt.Printf("Session until: %s (renewed automatically)\n", acct.ExpiresAt.Local().Format("2006-01-02"))
	}
	return nil
}

func runAccountDelete(args []string, in io.Reader, out io.Writer) error {
	fs := flag.NewFlagSet("account delete", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}
	backend, tokenPath, err := hostedBackendFromConfig()
	if err != nil {
		return err
	}
	if !*yes {
		acct, err := backend.Account()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "This permanently deletes the hosted account %s and all synced tasks.\n", acct.Email)
		fmt.Fprint(out, "Type DELETE to confirm: ")
		line, _ := bufio.NewReader(in).ReadString('\n')
		if strings.TrimSpace(line) != "DELETE" {
			return errors.New("aborted")
		}
	}
	if err := backend.DeleteAccount(); err != nil {
		return err
	}
	if err := internal.DeleteHostedToken(tokenPath); err != nil {
		return err
	}
	fmt.Fprintln(out, "Hosted account deleted. Your local task file was kept.")
	return nil
}
