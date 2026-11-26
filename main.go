package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/undont/jiru/internal/cli"
	"github.com/undont/jiru/internal/client"
	"github.com/undont/jiru/internal/config"
	"github.com/undont/jiru/internal/demo"
	"github.com/undont/jiru/internal/filters"
	"github.com/undont/jiru/internal/jira"
	"github.com/undont/jiru/internal/recents"
	"github.com/undont/jiru/internal/resume"
	"github.com/undont/jiru/internal/ui"
	"github.com/undont/jiru/internal/validate"
)

var version = "dev"

// profileFlag is the global --profile flag value.
var profileFlag string

func main() {
	var reset bool
	var demoMode bool
	var keyFlag string

	rootCmd := &cobra.Command{
		Use:     "jiru",
		Short:   "A terminal UI for Jira",
		Version: version,
		// Silence cobra's default error/usage printing — we handle it ourselves.
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if reset {
				return runReset()
			}
			if demoMode || os.Getenv("JIRU_DEMO") == "1" {
				return runDemo()
			}
			if keyFlag != "" {
				if err := validate.IssueKey(keyFlag); err != nil {
					return err
				}
			}
			return runTUI(keyFlag)
		},
	}

	rootCmd.Flags().BoolVarP(&reset, "reset", "r", false, "reset all config and credentials, then start the setup wizard")
	rootCmd.Flags().BoolVar(&demoMode, "demo", false, "launch the in-memory demo (no real Jira credentials required)")
	rootCmd.Flags().StringVarP(&keyFlag, "key", "k", "", "open an issue directly")
	rootCmd.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "use a named profile")

	// Register --profile flag completion from profiles.json entries.
	_ = rootCmd.RegisterFlagCompletionFunc("profile", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		names, err := config.ListProfileNames()
		if err != nil || len(names) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return names, cobra.ShellCompDirectiveNoFileComp
	})

	// CLI subcommands share a PersistentPreRunE for config/client init.
	cliGroup := &cobra.Group{ID: "cli", Title: "CLI Commands:"}
	rootCmd.AddGroup(cliGroup)

	getCmd := cli.GetCmd()
	searchCmd := cli.SearchCmd()
	listCmd := cli.ListCmd()
	boardsCmd := cli.BoardsCmd()
	wikiCmd := cli.WikiCmd()
	editCmd := cli.EditCmd()
	commentCmd := cli.CommentCmd()
	transitionCmd := cli.TransitionCmd()

	for _, cmd := range []*cobra.Command{getCmd, searchCmd, listCmd, boardsCmd, wikiCmd, editCmd, commentCmd, transitionCmd} {
		cmd.GroupID = "cli"
		cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
			return cli.InitClientWithProfile(profileFlag)
		}
	}

	rootCmd.AddCommand(getCmd, searchCmd, listCmd, boardsCmd, wikiCmd, editCmd, commentCmd, transitionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runReset() error {
	fmt.Print("This will remove all config and credentials. Continue? [y/N] ")
	var answer string
	_, _ = fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Cancelled.")
		return nil
	}
	if err := config.ResetConfig(); err != nil {
		return fmt.Errorf("resetting config: %w", err)
	}
	fmt.Println("Config and credentials cleared. Starting setup wizard...")

	empty := &config.Config{AuthType: "basic"}
	allMissing := []string{"domain", "user", "api_token"}
	p := tea.NewProgram(
		ui.NewApp(nil, "", empty, allMissing, version),
		tea.WithAltScreen(),
	)
	_, err := p.Run()
	return err
}

func runTUI(directIssue string) error {
	// Set the filter and recents profile if specified.
	if profileFlag != "" {
		filters.SetProfile(profileFlag)
		recents.SetProfile(profileFlag)
	} else {
		filters.SetProfile(config.ActiveProfileName())
		recents.SetProfile(config.ActiveProfileName())
	}

	partial, missing := config.PartialLoadProfile(profileFlag)

	var c client.JiraClient
	if len(missing) == 0 {
		cfg, err := config.LoadProfile(profileFlag)
		if err != nil {
			return fmt.Errorf("configuration error: %w", err)
		}
		c = client.New(cfg)
	}

	// Clear sensitive env vars after all config loading is complete
	// to prevent leakage to child processes (e.g. git).
	config.ClearSensitiveEnv()

	app := ui.NewApp(c, directIssue, partial, missing, version)
	if profileFlag != "" {
		app.SetProfileName(profileFlag)
	} else {
		app.SetProfileName(config.ActiveProfileName())
	}

	osc7577 := os.Getenv("OSC7577_TOKEN")
	_ = os.Unsetenv("OSC7577_TOKEN")

	p := tea.NewProgram(app, tea.WithAltScreen())
	if osc7577 != "" {
		_ = resume.Report(osc7577, resumeArgv(os.Args[1:]))
	}

	_, err := p.Run()
	// a SIGTERM returns an error and a SIGHUP never gets here, so only a quit clears
	if err == nil && osc7577 != "" {
		_ = resume.Clear(osc7577)
	}

	return err
}

// resumeArgv is jiru plus the profile and key flags as typed: the terminal
// compares the reported flags with the command line
func resumeArgv(args []string) []string {
	argv := []string{"jiru"}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if resumeFlags[arg] && i+1 < len(args) {
			argv = append(argv, arg, args[i+1])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--profile=") || strings.HasPrefix(arg, "--key=") {
			argv = append(argv, arg)
		}
	}
	return argv
}

var resumeFlags = map[string]bool{"-p": true, "--profile": true, "-k": true, "--key": true}

// runDemo launches the app against the in-memory demo client. No real config
// or credentials are read; saved filters and recents live under a dedicated
// profile slug so the user's real data is left alone. The version string is
// suppressed so the recorded gif ages gracefully across releases.
func runDemo() error {
	filters.SetProfile(demo.ProfileName)
	recents.SetProfile(demo.ProfileName)

	if err := seedDemoFilters(); err != nil {
		return fmt.Errorf("seeding demo filters: %w", err)
	}

	c := demo.New()
	app := ui.NewApp(c, "", c.Config(), nil, "")
	app.SetProfileName(demo.ProfileName)

	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// seedDemoFilters writes the curated demo filter set on every --demo boot so
// the filterpickview always shows the same three filters even if the user
// has tinkered with the file between runs.
func seedDemoFilters() error {
	for _, existing := range mustLoadFilters() {
		if err := filters.Delete(existing.ID); err != nil {
			return err
		}
	}
	for _, f := range demo.SeedSavedFilters() {
		saved, err := filters.Add(f.Name, f.JQL)
		if err != nil {
			return err
		}
		if f.Favourite {
			if err := filters.ToggleFavourite(saved.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func mustLoadFilters() []jira.SavedFilter {
	fs, _ := filters.Load()
	return fs
}
