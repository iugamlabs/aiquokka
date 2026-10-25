package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iugamlabs/aiquokka/internal/credential"
	"github.com/iugamlabs/aiquokka/internal/httpx"
	"github.com/iugamlabs/aiquokka/internal/provider"
	"github.com/iugamlabs/aiquokka/internal/usage"
	"gopkg.in/yaml.v3"
)

// structured reports whether a machine-readable format was requested.
func structured() bool { return jsonOut || yamlOut }

// emit writes v to stdout as JSON or YAML per the output flags.
func emit(v any) error {
	if yamlOut {
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err := enc.Encode(v); err != nil {
			return err
		}
		return enc.Close()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// withCredentialPolicy attaches the global --credential-policy to ctx.
func withCredentialPolicy(ctx context.Context) context.Context {
	policy := credentialPolicy
	if noRefresh {
		policy = credential.ReadOnly
	}
	return credential.WithPolicy(ctx, policy)
}

// run executes a single provider fetch and renders the result honoring --json.
// With --watch, it refreshes every watchInterval until interrupted.
func run(p provider.Provider) error {
	return maybeWatch(func(parent context.Context) error {
		report, err := fetchSingle(parent, p)
		if err != nil {
			return err
		}

		if structured() {
			return emit(report)
		}
		usage.Render(os.Stdout, report, time.Now())
		return nil
	})
}

// fetchSingle retries exactly once after a successful interactive official-CLI
// login. It is deliberately only used by an explicitly selected provider;
// aggregate, structured, watch, CI, and non-TTY commands never start login.
func fetchSingle(parent context.Context, p provider.Provider) (*usage.Report, error) {
	fetch := func() (*usage.Report, error) {
		ctx, cancel := context.WithTimeout(withCredentialPolicy(parent), 20*time.Second)
		defer cancel()
		return p.Fetch(ctx)
	}

	report, err := fetch()
	if err == nil || !mayInteractiveReauth() {
		return report, err
	}
	var reauth *credential.ReauthRequiredError
	if !errors.As(err, &reauth) || !isProviderLoginCommand(p, reauth.Command) {
		return report, err
	}
	if err := runOfficialLogin(parent, reauth.Command); err != nil {
		return nil, fmt.Errorf("%s sign-in failed: %w", p.Name(), err)
	}
	return fetch()
}

// mayInteractiveReauth encodes the narrow circumstances under which it is OK
// to hand the terminal to an official CLI (which may open a browser).
func mayInteractiveReauth() bool {
	return !watch && !structured() && os.Getenv("CI") == "" && stdinIsTTY() && stdoutIsTTY()
}

func isProviderLoginCommand(p provider.Provider, command string) bool {
	parts := strings.Fields(command)
	return len(parts) == 2 && parts[1] == "login" && parts[0] == p.ID() &&
		(parts[0] == "codex" || parts[0] == "grok")
}

// runOfficialLogin is a variable so command-level recovery can be tested
// without spawning a real login flow.
var runOfficialLogin = func(ctx context.Context, command string) error {
	parts := strings.Fields(command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// maybeWatch runs fn once, or repeatedly every watchInterval when --watch is set.
// Ctrl+C / SIGTERM stops cleanly (exit 0). Interrupt during a fetch cancels it.
// If work is stuck outside the cancelled context (a Keychain dialog), a second
// Ctrl+C or a short grace period force-exits.
func maybeWatch(fn func(ctx context.Context) error) error {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigs)

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-sigs:
			stop()
			select {
			case <-sigs:
				os.Exit(130)
			case <-done:
			case <-time.After(stuckInterruptGrace):
				os.Exit(130)
			}
		case <-ctx.Done():
		}
	}()

	return watchLoop(ctx, watch, watchInterval, fn)
}

// stuckInterruptGrace is how long we wait after Ctrl+C for in-flight work to
// unwind before killing the process. Long enough for a context-aware fetch to
// return; short enough that a Keychain dialog cannot trap the terminal.
const stuckInterruptGrace = 500 * time.Millisecond

// watchLoop is the core of --watch. enabled=false runs fn once.
// ctx cancellation (e.g. Ctrl+C) ends the loop with a nil error.
// On a TTY the wait shows a pulsating countdown; press r to refresh early.
func watchLoop(ctx context.Context, enabled bool, interval time.Duration, fn func(ctx context.Context) error) error {
	if !enabled {
		return fn(ctx)
	}

	for {
		if err := fn(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		switch waitForRefresh(ctx, interval) {
		case watchStop:
			return nil
		case watchTick, watchManual:
			clearWatchFrame()
		}
	}
}

// clearWatchFrame prepares the terminal for the next --watch refresh.
func clearWatchFrame() {
	if structured() {
		return
	}
	if stdoutIsTTY() {
		// Home cursor and clear the screen so the next frame replaces the last.
		fmt.Fprint(os.Stdout, "\033[H\033[2J")
		return
	}
	fmt.Fprintln(os.Stdout)
}

// fetchResult is one provider's outcome in the aggregate view.
type fetchResult struct {
	name   string
	report *usage.Report
	err    error
}

// runAll fetches every provider concurrently. On a TTY it shows a fixed-order
// skeleton for configured providers and rewrites the view as each result
// arrives. Non-TTY and --json/--yaml wait for every fetch, then emit once.
// With --watch, the whole view refreshes every watchInterval until interrupted.
func runAll(providers []provider.Provider) error {
	return maybeWatch(func(parent context.Context) error {
		ctx, cancel := context.WithTimeout(withCredentialPolicy(parent), 30*time.Second)
		defer cancel()

		if structured() {
			return runAllStructured(ctx, providers)
		}
		if stdoutIsTTY() {
			return runAllLive(ctx, providers)
		}
		return runAllBatch(ctx, providers)
	})
}

func runAllStructured(ctx context.Context, providers []provider.Provider) error {
	results := fetchAnswered(ctx, providers)
	out := map[string]any{}
	for _, r := range results {
		switch {
		case r.err == nil:
			out[strings.ToLower(r.name)] = r.report
		case usage.IsNotConfigured(r.err):
			// Skip providers the user doesn't use.
		default:
			out[strings.ToLower(r.name)] = map[string]string{"error": r.err.Error()}
		}
	}
	return emit(out)
}

// runAllBatch waits for every provider, then prints in fixed provider order.
// Used when stdout is not a TTY (pipes, redirects).
func runAllBatch(ctx context.Context, providers []provider.Provider) error {
	results := fetchAnswered(ctx, providers)
	now := time.Now()

	var reports []*usage.Report
	for _, r := range results {
		if r.err == nil {
			reports = append(reports, r.report)
		}
	}
	labelWidth := usage.LabelWidth(reports...)

	shown := 0
	for _, r := range results {
		if r.err != nil && usage.IsNotConfigured(r.err) {
			continue
		}
		if shown > 0 {
			fmt.Fprintln(os.Stdout)
		}
		shown++
		if r.err != nil {
			writeProviderError(os.Stdout, r.name, r.err)
			continue
		}
		usage.RenderAligned(os.Stdout, r.report, now, labelWidth)
	}
	if shown == 0 {
		fmt.Fprintln(os.Stdout, "No configured providers found. Log in to at least one provider.")
	}
	return nil
}

// configProbeGrace is how long we wait for cheap NotConfigured probes before
// painting the first skeleton, so providers the user doesn't use rarely flash.
const configProbeGrace = 80 * time.Millisecond

// liveTitle gives the interactive aggregate view a little of aiquokka's
// personality. It deliberately belongs only to the TTY renderer: piped and
// structured output must remain easy for people and programs to consume.
const liveTitle = "🦘 AIQuokka — curious, helpful, happily shipping"

// slot is one provider's live-view state.
type slot struct {
	name   string
	done   bool
	skip   bool // not configured — omit from the view
	report *usage.Report
	err    error
}

// runAllLive paints a fixed-order skeleton, then rewrites the view in place as
// each provider finishes. Unconfigured providers are dropped as soon as their
// fetch reports NotConfigured.
func runAllLive(ctx context.Context, providers []provider.Provider) error {
	slots := make([]slot, len(providers))
	for i, p := range providers {
		slots[i].name = p.Name()
	}

	type event struct {
		i      int
		report *usage.Report
		err    error
	}
	ch := make(chan event, len(providers))
	for i, p := range providers {
		go func(i int, p provider.Provider) {
			r, err := p.Fetch(ctx)
			ch <- event{i: i, report: r, err: err}
		}(i, p)
	}

	now := time.Now()
	printedLines := 0
	remaining := len(providers)
	painted := false

	grace := time.NewTimer(configProbeGrace)
	defer grace.Stop()
	var graceC <-chan time.Time = grace.C

	paint := func() {
		printedLines = paintLive(os.Stdout, slots, now, printedLines)
		painted = true
	}

	for remaining > 0 {
		select {
		case ev := <-ch:
			remaining--
			s := &slots[ev.i]
			s.done = true
			if usage.IsNotConfigured(ev.err) {
				s.skip = true
			} else {
				s.report = ev.report
				s.err = ev.err
			}
			// Paint once the grace window has opened, or immediately when the
			// last fetch returns (so a single slow provider still shows).
			if painted || graceC == nil || remaining == 0 {
				paint()
			}
		case <-graceC:
			graceC = nil
			paint()
		}
	}
	// If every provider was unconfigured and grace never forced a paint with
	// content, ensure the empty-state message is shown.
	if !painted {
		paint()
	}
	return nil
}

// paintLive rewrites the aggregate view from the top of the previous frame.
// Returns the number of lines just printed (for the next cursor-up).
func paintLive(w io.Writer, slots []slot, now time.Time, prevLines int) int {
	var body strings.Builder
	writeLiveBody(&body, slots, now)
	content := body.String()

	if prevLines > 0 {
		// Move to the start of the previous frame and clear downward.
		fmt.Fprintf(w, "\033[%dA\033[J", prevLines)
	}
	fmt.Fprint(w, content)

	if content == "" {
		return 0
	}
	return strings.Count(content, "\n")
}

// writeLiveBody renders the current fixed-order view into b.
func writeLiveBody(b *strings.Builder, slots []slot, now time.Time) {
	b.WriteString(liveTitle)
	b.WriteString("\n\n")

	var reports []*usage.Report
	for _, s := range slots {
		if s.done && !s.skip && s.err == nil && s.report != nil {
			reports = append(reports, s.report)
		}
	}
	labelWidth := usage.LabelWidth(reports...)

	shown := 0
	allDone := true
	for _, s := range slots {
		if s.skip {
			continue
		}
		if !s.done {
			allDone = false
		}
		if shown > 0 {
			b.WriteByte('\n')
		}
		shown++
		switch {
		case !s.done:
			writeSkeleton(b, s.name)
		case s.err != nil:
			writeProviderError(b, s.name, s.err)
		default:
			usage.RenderAligned(b, s.report, now, labelWidth)
		}
	}
	if shown == 0 && allDone {
		b.WriteString("No configured providers found. Log in to at least one provider.\n")
	}
}

func writeSkeleton(w io.Writer, name string) {
	fmt.Fprintln(w, name)
	fmt.Fprintln(w, strings.Repeat("─", len(name)))
	fmt.Fprintln(w, "  …")
}

func writeProviderError(w io.Writer, name string, err error) {
	fmt.Fprintln(w, name)
	fmt.Fprintln(w, strings.Repeat("─", len(name)))
	var reauth *credential.ReauthRequiredError
	if errors.As(err, &reauth) && reauth.Command != "" {
		fmt.Fprintf(w, "  Sign-in required — run `%s`\n", reauth.Command)
		return
	}
	fmt.Fprintf(w, "  %s\n", err.Error())
}

// fetchAll runs every provider concurrently and returns results in the same
// order as providers.
func fetchAll(ctx context.Context, providers []provider.Provider) []fetchResult {
	return fetchWithin(ctx, providers, 0)
}

// fetchAnswered bounds one-shot output by the shared HTTP timeout.
func fetchAnswered(ctx context.Context, providers []provider.Provider) []fetchResult {
	return fetchWithin(ctx, providers, httpx.Client.Timeout)
}

// fetchWithin preserves provider order and cancels late work. A buffered channel
// lets even a provider ignoring cancellation finish without blocking a sender.
func fetchWithin(ctx context.Context, providers []provider.Provider, deadline time.Duration) []fetchResult {
	parent := ctx
	var cancel context.CancelFunc
	if deadline > 0 {
		ctx, cancel = context.WithTimeout(ctx, deadline)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	type answer struct {
		i      int
		report *usage.Report
		err    error
	}
	ch := make(chan answer, len(providers))
	results := make([]fetchResult, len(providers))
	done := make([]bool, len(providers))
	for i, p := range providers {
		results[i].name = p.Name()
		go func(i int, p provider.Provider) { r, err := p.Fetch(ctx); ch <- answer{i, r, err} }(i, p)
	}
	accept := func(a answer) {
		results[a.i] = fetchResult{name: providers[a.i].Name(), report: a.report, err: a.err}
		done[a.i] = true
	}
	for remaining := len(providers); remaining > 0; remaining-- {
		select {
		case a := <-ch:
			accept(a)
		case <-ctx.Done():
			// Keep answers already delivered before reporting the unfinished providers.
			for {
				select {
				case a := <-ch:
					accept(a)
				default:
					goto drained
				}
			}
		drained:
			err := ctx.Err()
			if parent.Err() == nil && deadline > 0 {
				err = fmt.Errorf("did not answer within %s", deadline)
			}
			for i := range results {
				if !done[i] {
					results[i].err = err
				}
			}
			return results
		}
	}
	return results
}

func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
