package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gammons/slk/internal/cache"
	"github.com/gammons/slk/internal/config"
	"github.com/gammons/slk/internal/core"
	slackclient "github.com/gammons/slk/internal/slack"
	"github.com/gammons/slk/internal/slackurl"
	"github.com/gammons/slk/internal/text"
	"github.com/gammons/slk/internal/ui/sidebar"
	"github.com/slack-go/slack"
)

const (
	envSlkChannel    = "SLK_CHANNEL"
	envSlkThread     = "SLK_THREAD"
	envSlkFullscreen = "SLK_FULLSCREEN"
)

var (
	bareTSRegex       = regexp.MustCompile(`^\d+\.\d+$`)
	rawChannelIDRegex = regexp.MustCompile(`^[CDG][A-Z0-9]+$`)
)

// LaunchTarget represents the parsed and merged target specification for launch.
type LaunchTarget struct {
	WorkspaceHint   string // Subdomain from permalink (e.g. "truelist-workspace")
	ChannelID       string // Channel ID if raw form or from permalink (e.g. "C0123...")
	ChannelName     string // Channel name without leading '#' (e.g. "general")
	RawChannel      string // Original channel argument
	ThreadTS        string // Thread timestamp
	SelectMessageTS string // Message timestamp for select-message permalink form (no thread_ts)
	Fullscreen      bool   // Whether fullscreen zoom is requested

	ChannelSource    string // "flag", "env", or ""
	ThreadSource     string // "flag", "env", or ""
	FullscreenSource string // "flag", "env", or ""

	ConflictNotice string // Notice when thread permalink channel overrides explicit channel
	Notice         string // Composed notice (for failures, fallback, or provenance)

	PinnedTeamID string // Resolved TeamID from WorkspaceHint
}

// EarlyArgAction indicates how argv was handled by the early dispatcher.
type EarlyArgAction int

const (
	EarlyArgLaunchTUI EarlyArgAction = iota
	EarlyArgHandled
)

type switchHandlers struct {
	addWorkspace    func() error
	removeWorkspace func() error
	listWorkspaces  func() error
	dumpSections    func() error
	dumpPrefs       func() error
	hasWorkspaces   func() bool
}

func defaultSwitchHandlers() *switchHandlers {
	return &switchHandlers{
		addWorkspace:    addWorkspace,
		removeWorkspace: removeWorkspace,
		listWorkspaces:  listWorkspaces,
		dumpSections:    dumpSections,
		dumpPrefs:       dumpPrefs,
		hasWorkspaces:   func() bool { return hasConfiguredWorkspaces(xdgData()) },
	}
}

// readLaunchEnv reads the three SLK_* environment variables.
// This is the single read site in the codebase for these variables (R8).
func readLaunchEnv() (channel, thread, fullscreen string) {
	return os.Getenv("SLK_CHANNEL"), os.Getenv("SLK_THREAD"), os.Getenv("SLK_FULLSCREEN")
}

// parseEnvFullscreen parses the SLK_FULLSCREEN environment variable.
// Accepted truthy set: "1", "true", "yes", "on" (case-folded).
// Everything else non-empty: false.
// Unset / empty-after-trim: absent (isSet=false).
func parseEnvFullscreen(val string) (value bool, isSet bool) {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return false, false
	}
	switch strings.ToLower(trimmed) {
	case "1", "true", "yes", "on":
		return true, true
	default:
		return false, true
	}
}

// parseLaunchArgs parses argv and environment variables.
// It handles single-word switches (--version, --help, etc.) as well as the new
// flags (--channel, --thread, --fullscreen).
// Returns (action, exitCode, target, err).
func parseLaunchArgs(
	argv []string,
	stdout, stderr io.Writer,
	getenv func(string) string,
	handlers *switchHandlers,
) (EarlyArgAction, int, *LaunchTarget, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	if handlers == nil {
		handlers = defaultSwitchHandlers()
	}

	// 1. Check existing single-word switches byte-identically (cli-switch-compat)
	if len(argv) > 0 {
		switch argv[0] {
		case "--version", "-v", "version":
			fmt.Fprintf(stdout, "slk %s (commit %s, built %s)\n", version, commit, date)
			fmt.Fprintln(stdout, "Unofficial Slack client. Not affiliated with Slack Technologies, LLC.")
			fmt.Fprintln(stdout, "Uses Slack's internal browser protocol; may violate Slack's TOS. Use at your own risk.")
			return EarlyArgHandled, 0, nil, nil
		case "--help", "-h", "help":
			printHelpTo(stdout)
			return EarlyArgHandled, 0, nil, nil
		case "--add-workspace":
			if err := handlers.addWorkspace(); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return EarlyArgHandled, 1, nil, err
			}
			return EarlyArgHandled, 0, nil, nil
		case "--remove-workspace":
			if err := handlers.removeWorkspace(); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return EarlyArgHandled, 1, nil, err
			}
			return EarlyArgHandled, 0, nil, nil
		case "--list-workspaces":
			if err := handlers.listWorkspaces(); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return EarlyArgHandled, 1, nil, err
			}
			return EarlyArgHandled, 0, nil, nil
		case "--dump-sections":
			if err := handlers.dumpSections(); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return EarlyArgHandled, 1, nil, err
			}
			return EarlyArgHandled, 0, nil, nil
		case "--dump-prefs":
			if err := handlers.dumpPrefs(); err != nil {
				fmt.Fprintf(stderr, "Error: %v\n", err)
				return EarlyArgHandled, 1, nil, err
			}
			return EarlyArgHandled, 0, nil, nil
		}
	}

	// 2. Parse flags layer via stdlib flag.FlagSet (cli-flags-parse)
	fs := flag.NewFlagSet("slk", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		printHelpTo(stderr)
	}

	var flagChannel, flagThread string
	var flagFullscreen bool
	fs.StringVar(&flagChannel, "channel", "", "Target channel (id, name, or permalink)")
	fs.StringVar(&flagThread, "thread", "", "Target thread (ts or permalink)")
	fs.BoolVar(&flagFullscreen, "fullscreen", false, "Launch with pane zoomed immediately")

	if err := fs.Parse(argv); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printHelpTo(stdout)
			return EarlyArgHandled, 0, nil, nil
		}
		return EarlyArgHandled, 2, nil, err
	}

	// Reject unexpected positional arguments after flag parsing (lp-positional-reject)
	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "Error: unexpected positional argument %q\n", fs.Args()[0])
		printHelpTo(stderr)
		return EarlyArgHandled, 2, nil, fmt.Errorf("unexpected positional argument: %s", fs.Args()[0])
	}

	// Check if a flag value starts with dash (e.g. --channel --fullscreen) (lp-positional-reject)
	if strings.HasPrefix(flagChannel, "-") {
		fmt.Fprintf(stderr, "Error: invalid value for --channel: %q\n", flagChannel)
		printHelpTo(stderr)
		return EarlyArgHandled, 2, nil, fmt.Errorf("invalid value for --channel: %s", flagChannel)
	}
	if strings.HasPrefix(flagThread, "-") {
		fmt.Fprintf(stderr, "Error: invalid value for --thread: %q\n", flagThread)
		printHelpTo(stderr)
		return EarlyArgHandled, 2, nil, fmt.Errorf("invalid value for --thread: %s", flagThread)
	}

	flagFullscreenSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "fullscreen" {
			flagFullscreenSet = true
		}
	})

	// 3. Read environment variables (cli-env-equivalents, R8)
	var envCh, envTh, envFs string
	if getenv != nil {
		envCh = getenv(envSlkChannel)
		envTh = getenv(envSlkThread)
		envFs = getenv(envSlkFullscreen)
	}

	chVal := strings.TrimSpace(flagChannel)
	thVal := strings.TrimSpace(flagThread)
	envCh = strings.TrimSpace(envCh)
	envTh = strings.TrimSpace(envTh)
	envFsBool, envFsSet := parseEnvFullscreen(envFs)

	var effectiveChannel, effectiveThread string
	var chSource, thSource string

	if chVal != "" {
		effectiveChannel = chVal
		chSource = "flag"
	} else if envCh != "" {
		effectiveChannel = envCh
		chSource = "env"
	}

	if thVal != "" {
		effectiveThread = thVal
		thSource = "flag"
	} else if envTh != "" {
		effectiveThread = envTh
		thSource = "env"
	}

	var effectiveFullscreen bool
	var fsSource string
	if flagFullscreenSet {
		effectiveFullscreen = flagFullscreen
		fsSource = "flag"
	} else if envFsSet {
		effectiveFullscreen = envFsBool
		fsSource = "env"
	}

	// If no flags and no ENV were specified, return launch unpinned
	if effectiveChannel == "" && effectiveThread == "" && !effectiveFullscreen {
		return EarlyArgLaunchTUI, 0, nil, nil
	}

	// 4. R10 validation across merged target (lp-r10-env-merge)
	// If effectiveThread is specified (and is not a permalink that carries a channel):
	isThreadPermalink := false
	if effectiveThread != "" {
		if _, ok := slackurl.Parse(effectiveThread); ok {
			isThreadPermalink = true
		}
	}

	if effectiveThread != "" && !isThreadPermalink && effectiveChannel == "" {
		if thSource == "flag" {
			// Flag-supplied bare --thread with no channel -> exit 2
			fmt.Fprintln(stderr, "Error: --thread requires --channel")
			printHelpTo(stderr)
			return EarlyArgHandled, 2, nil, errors.New("--thread requires --channel")
		}
		// ENV-supplied thread with no effective channel -> soft fallback (exit 0, launch normal with notice)
		target := &LaunchTarget{
			Notice:           "thread ignored: no channel specified via SLK_CHANNEL or --channel",
			Fullscreen:       effectiveFullscreen,
			FullscreenSource: fsSource,
		}
		return EarlyArgLaunchTUI, 0, target, nil
	}

	// 5. Pure target resolution (target-pure-parse)
	target, err := resolveLaunchTarget(effectiveChannel, effectiveThread, effectiveFullscreen, chSource, thSource, fsSource)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		printHelpTo(stderr)
		return EarlyArgHandled, 2, nil, err
	}

	// If zero workspaces are configured, drop the pin silently at parse time (lp-onboarding-drop)
	hasWS := handlers.hasWorkspaces
	if hasWS == nil {
		hasWS = func() bool { return hasConfiguredWorkspaces(xdgData()) }
	}
	if !hasWS() {
		return EarlyArgLaunchTUI, 0, nil, nil
	}

	return EarlyArgLaunchTUI, 0, target, nil
}

// hasConfiguredWorkspaces returns true if at least one workspace token is configured on disk.
func hasConfiguredWorkspaces(dataDir string) bool {
	if dataDir == "" {
		return false
	}
	tokenDir := filepath.Join(dataDir, "tokens")
	tokenStore := slackclient.NewTokenStore(tokenDir)
	tokens, err := tokenStore.List()
	return err == nil && len(tokens) > 0
}

// resolveLaunchTarget purely parses and classifies channel, thread, and fullscreen targets.
func resolveLaunchTarget(
	rawChannel, rawThread string,
	fullscreen bool,
	chSource, thSource, fsSource string,
) (*LaunchTarget, error) {
	target := &LaunchTarget{
		RawChannel:       rawChannel,
		Fullscreen:       fullscreen,
		ChannelSource:    chSource,
		ThreadSource:     thSource,
		FullscreenSource: fsSource,
	}

	var conflictNotice string

	// Parse thread target first if present
	if rawThread != "" {
		if pl, ok := slackurl.Parse(rawThread); ok {
			target.WorkspaceHint = pl.Subdomain
			threadChannelID := string(pl.ChannelID)
			target.ChannelID = threadChannelID

			if pl.ThreadTS != "" {
				target.ThreadTS = string(pl.ThreadTS)
			} else if pl.MessageTS != "" {
				target.SelectMessageTS = string(pl.MessageTS)
			}

			// Check cross-source conflict: if rawChannel was also provided and differs (lp-cross-source-conflict)
			if rawChannel != "" {
				explicitChan := rawChannel
				if strings.HasPrefix(explicitChan, "#") {
					explicitChan = explicitChan[1:]
				}
				if plChan, ok := slackurl.Parse(rawChannel); ok {
					explicitChan = string(plChan.ChannelID)
				}
				if !strings.EqualFold(explicitChan, threadChannelID) {
					conflictNotice = fmt.Sprintf("channel conflict: %s from permalink overrides %s", threadChannelID, explicitChan)
				}
			}
		} else if bareTSRegex.MatchString(rawThread) {
			target.ThreadTS = rawThread
		} else if strings.HasPrefix(rawThread, "http://") || strings.HasPrefix(rawThread, "https://") || strings.Contains(rawThread, ".slack.com") {
			// Syntactically flagged as permalink, but Parse failed -> R2 fallback + notice (gotcha batch 2 #11)
			target.Notice = "thread permalink unresolvable: " + sanitizeNoticeToken(rawThread)
		} else {
			// Neither permalink nor valid ts syntax -> syntax error exit 2
			return nil, fmt.Errorf("invalid thread timestamp: %q", rawThread)
		}
	}

	// Parse channel target if not already pinned by thread permalink
	if rawChannel != "" && target.ChannelID == "" {
		if pl, ok := slackurl.Parse(rawChannel); ok {
			target.WorkspaceHint = pl.Subdomain
			target.ChannelID = string(pl.ChannelID)
			if target.ThreadTS == "" {
				if pl.ThreadTS != "" {
					target.ThreadTS = string(pl.ThreadTS)
				} else if pl.MessageTS != "" {
					target.SelectMessageTS = string(pl.MessageTS)
				}
			}
		} else {
			stripped := rawChannel
			if strings.HasPrefix(stripped, "#") {
				stripped = stripped[1:]
			}
			if rawChannelIDRegex.MatchString(stripped) {
				target.ChannelID = stripped
			} else {
				target.ChannelName = stripped
			}
		}
	}

	if conflictNotice != "" {
		target.ConflictNotice = conflictNotice
		target.Notice = composeNotice(target.Notice, conflictNotice)
	}

	// Provenance notice for successful ENV pins (lp-pin-provenance)
	if target.Notice == "" {
		if chSource == "env" || thSource == "env" {
			token := target.ChannelName
			if token != "" {
				token = "#" + token
			} else if target.ChannelID != "" {
				token = target.ChannelID
			} else {
				token = target.ThreadTS
			}
			src := "SLK_CHANNEL"
			if chSource != "env" && thSource == "env" {
				src = "SLK_THREAD"
			}
			target.Notice = fmt.Sprintf("pinned to %s via %s", sanitizeNoticeToken(token), src)
		}
	}

	return target, nil
}

// sanitizeNoticeToken strips control characters and trims long permalink prefixes.
func sanitizeNoticeToken(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)
	if len(s) > 40 {
		return s[:37] + "…"
	}
	return s
}

// composeNotice combines failure/fallback notices into a single string.
func composeNotice(existing, added string) string {
	if existing == "" {
		return added
	}
	if added == "" {
		return existing
	}
	return existing + "; " + added
}

// matchWorkspaceBySubdomain matches a permalink subdomain against configured workspaces.
// Checks token-store Domain first, then cache Workspace.Domain fallback, case-folded (lp-workspace-domain-source).
func matchWorkspaceBySubdomain(subdomain string, tokens []slackclient.Token, cacheWorkspaces []cache.Workspace) (string, bool) {
	sub := strings.TrimSpace(subdomain)
	if sub == "" {
		return "", false
	}

	// 1. Token store Domain
	for _, tok := range tokens {
		if tok.Domain != "" && strings.EqualFold(tok.Domain, sub) {
			return tok.TeamID, true
		}
	}

	// 2. Cache Workspace Domain
	for _, ws := range cacheWorkspaces {
		if ws.Domain != "" && strings.EqualFold(ws.Domain, sub) {
			return ws.ID, true
		}
	}

	return "", false
}

// resolveChannelTarget resolves the target channel against the local channel list,
// falling back to one bounded GetConversationInfo call for raw-ID misses (target-channel-resolve, R11).
func resolveChannelTarget(
	ctx context.Context,
	target *LaunchTarget,
	channels []sidebar.ChannelItem,
	getConv func(context.Context, string) (*slack.Channel, error),
) (resolvedID string, unlisted *slack.Channel, notice string) {
	if target == nil {
		return "", nil, ""
	}

	// 1. Raw Channel ID lookup
	if target.ChannelID != "" {
		for _, ch := range channels {
			if ch.ID == target.ChannelID {
				return ch.ID, nil, ""
			}
		}

		// Raw ID miss: attempt one bounded GetConversationInfo (lp-resolve-timeout)
		if getConv != nil {
			timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			conv, err := getConv(timeoutCtx, target.ChannelID)
			if err == nil && conv != nil {
				return target.ChannelID, conv, ""
			}
		}

		return "", nil, "channel not found: " + sanitizeNoticeToken(target.ChannelID)
	}

	// 2. Channel Name lookup (channels only, fold-matched, target-channel-resolve)
	if target.ChannelName != "" {
		targetFold := text.Fold(target.ChannelName)
		var matches []sidebar.ChannelItem
		for _, ch := range channels {
			// Name matching is scoped to public/private channels, not DMs/MPIMs
			if ch.Type != "channel" && ch.Type != "private" {
				continue
			}
			if text.Fold(ch.Name) == targetFold {
				matches = append(matches, ch)
			}
		}

		if len(matches) == 1 {
			return matches[0].ID, nil, ""
		}
		if len(matches) > 1 {
			return "", nil, "channel name ambiguous: " + sanitizeNoticeToken(target.ChannelName)
		}
		return "", nil, "channel not found: " + sanitizeNoticeToken(target.ChannelName)
	}

	return "", nil, ""
}

// hydrateUnlistedChannel creates sidebar and finder items for an unlisted channel (lp-resolved-unlisted-hydrate).
func hydrateUnlistedChannel(
	conv *slack.Channel,
	wctx *WorkspaceContext,
	cfg config.Config,
	teamID string,
) (sidebar.ChannelItem, core.ChannelFinderItem) {
	item, finderItem := buildChannelItem(*conv, wctx, cfg, teamID)
	return item, finderItem
}

// resolveAndHydrateLaunchTarget resolves the target channel against workspace channels and hydrates it if unlisted.
func resolveAndHydrateLaunchTarget(
	ctx context.Context,
	target *LaunchTarget,
	wctx *WorkspaceContext,
	cfg config.Config,
) (pinChID, pinThreadTS, pinMsgTS string, pinFullscreen bool, launchNotice string) {
	if target == nil {
		return "", "", "", false, ""
	}
	resolvedChID, unlistedConv, chNotice := resolveChannelTarget(ctx, target, wctx.Channels, func(c context.Context, id string) (*slack.Channel, error) {
		return wctx.Client.GetConversationInfo(c, id)
	})
	if unlistedConv != nil {
		chItem, finderItem := hydrateUnlistedChannel(unlistedConv, wctx, cfg, wctx.TeamID)
		wctx.Channels = append(wctx.Channels, chItem)
		wctx.FinderItems = append(wctx.FinderItems, finderItem)
	}
	pinChID = resolvedChID
	if resolvedChID != "" {
		pinThreadTS = target.ThreadTS
		pinMsgTS = target.SelectMessageTS
		pinFullscreen = target.Fullscreen
	} else {
		pinFullscreen = false
	}
	launchNotice = target.Notice
	if chNotice != "" {
		launchNotice = composeNotice(launchNotice, chNotice)
	}
	return
}

// printHelpTo emits usage information to the given writer (cli-help-doc).
func printHelpTo(w io.Writer) {
	fmt.Fprintf(w, `slk %s -- a blazingly fast Slack TUI

Usage:
  slk                    Launch the TUI
  slk --channel <target> [--thread <target>] [--fullscreen]
                         Launch pinned to channel/thread (zoomed if requested)
  slk --add-workspace     Add a Slack workspace (interactive)
  slk --remove-workspace  Remove a configured workspace (interactive)
  slk --list-workspaces   List configured workspaces (TeamID, Slug, Name)
  slk --dump-sections     Dump raw users.channelSections.list JSON (diagnostic)
  slk --dump-prefs        Dump client preferences JSON (diagnostic)
  slk --version          Print version and exit
  slk --help             Show this help

Launch flags:
  --channel <id|name|permalink>
                         Pin launch to channel ID, name (#name), or permalink
  --thread <ts|permalink>
                         Pin launch to thread ts or permalink (requires channel)
  --fullscreen           Launch with pane zoomed immediately

Environment variables:
  SLK_CHANNEL            Equivalent to --channel
  SLK_THREAD             Equivalent to --thread
  SLK_FULLSCREEN         Equivalent to --fullscreen (truthy: 1, true, yes, on)

Precedence: flags > ENV > persisted state / config.
Fallback: Unresolvable targets fall back to normal launch with a notice.

Config:  ~/.config/slk/config.toml
Data:    ~/.local/share/slk/
Cache:   ~/.cache/slk/

Docs:    https://github.com/gammons/slk
`, version)
}
