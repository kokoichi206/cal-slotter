package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/kokoichi206/cal-slotter/internal/config"
	"github.com/kokoichi206/cal-slotter/internal/googlecal"
	"github.com/kokoichi206/cal-slotter/internal/slotter"
	"github.com/kokoichi206/cal-slotter/internal/timefmt"
)

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type slotJSON struct {
	Start string `json:"start"`
	End   string `json:"end"`
	Text  string `json:"text,omitempty"`
}

type findJSON struct {
	Slots []slotJSON `json:"slots"`
}

type findOptions struct {
	Ranges          []string
	DurationMinutes int
	StepMinutes     int
	Count           int
	Strategy        slotter.SelectionStrategy
	Debug           bool
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("missing command")
	}

	switch args[0] {
	case "auth":
		return runAuth(args[1:], stdout, stderr)
	case "find":
		return runFind(args[1:], stdout, stderr)
	case "hold":
		return runHold(args[1:], stdin, stdout, stderr)
	case "confirm":
		return runConfirm(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runAuth(args []string, stdout, stderr io.Writer) error {
	defaultCredentials, err := config.DefaultCredentialsPath()
	if err != nil {
		return err
	}
	defaultToken, err := config.DefaultTokenPath()
	if err != nil {
		return err
	}

	fs := newFlagSet("schedule auth", stderr)
	credentialsPath := fs.String("credentials", defaultCredentials, "OAuth client credentials JSON path")
	tokenPath := fs.String("token", defaultToken, "OAuth token JSON path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return googlecal.Authenticate(context.Background(), *credentialsPath, *tokenPath, stdout)
}

func runFind(args []string, stdout, stderr io.Writer) error {
	cfg, err := loadConfigForCommand(args, stderr)
	if err != nil {
		return err
	}

	var ranges stringList
	fs := newFlagSet("schedule find", stderr)
	configureSharedFlags(fs, &cfg)
	fs.Var(&ranges, "range", "candidate window: YYYY-MM-DD HH:MM-HH:MM")
	durationMinutes := fs.Int("duration", 60, "meeting duration in minutes")
	stepMinutes := fs.Int("step", 30, "slot start step in minutes")
	count := fs.Int("count", 5, "maximum number of slots")
	strategy := fs.String("strategy", string(slotter.SelectionBalanced), "slot selection strategy: balanced or early")
	jsonOutput := fs.Bool("json", false, "output JSON")
	debugOutput := fs.Bool("debug", false, "output debug details to stderr")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err = applySharedFlagDefaults(cfg)
	if err != nil {
		return err
	}
	members := resolveMembers(cfg)
	if len(members) == 0 {
		return errors.New("members are required in config or --members")
	}
	if len(ranges) == 0 {
		return errors.New("at least one --range is required")
	}
	if *durationMinutes <= 0 {
		return errors.New("--duration must be greater than 0")
	}
	if *stepMinutes <= 0 {
		return errors.New("--step must be greater than 0")
	}
	if *count < 0 {
		return errors.New("--count must be greater than or equal to 0")
	}
	selectionStrategy, err := parseSelectionStrategy(*strategy)
	if err != nil {
		return err
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return err
	}
	windows, err := parseWindows(ranges, loc)
	if err != nil {
		return err
	}

	service, err := googlecal.NewService(context.Background(), cfg.Credentials, cfg.Token, cfg.CalendarID)
	if err != nil {
		return err
	}
	slots, err := findSlots(
		context.Background(),
		service,
		members,
		loc,
		findOptions{
			Ranges:          ranges,
			DurationMinutes: *durationMinutes,
			StepMinutes:     *stepMinutes,
			Count:           *count,
			Strategy:        selectionStrategy,
			Debug:           *debugOutput,
		},
		stderr,
	)
	if err != nil {
		return err
	}

	if *jsonOutput {
		return writeSlotsJSON(stdout, slots)
	}
	if len(slots) == 0 {
		fmt.Fprintf(stderr, "no available slots found: duration=%dmin step=%dmin windows=%d members=%d\n", *durationMinutes, *stepMinutes, len(windows), len(members))
		return nil
	}
	for _, slot := range slots {
		fmt.Fprintln(stdout, timefmt.FormatCustomerSlot(slot.In(loc)))
	}
	return nil
}

func runHold(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cfg, err := loadConfigForCommand(args, stderr)
	if err != nil {
		return err
	}

	var slotValues stringList
	var ranges stringList
	fs := newFlagSet("schedule hold", stderr)
	configureSharedFlags(fs, &cfg)
	title := fs.String("title", "", "case title")
	fs.Var(&slotValues, "slot", "hold slot: YYYY-MM-DD HH:MM-HH:MM")
	fs.Var(&ranges, "range", "candidate window: YYYY-MM-DD HH:MM-HH:MM")
	durationMinutes := fs.Int("duration", 60, "meeting duration in minutes")
	stepMinutes := fs.Int("step", 30, "slot start step in minutes")
	count := fs.Int("count", 5, "maximum number of slots")
	strategy := fs.String("strategy", string(slotter.SelectionBalanced), "slot selection strategy: balanced or early")
	debugOutput := fs.Bool("debug", false, "output debug details to stderr")
	sendUpdates := fs.Bool("send-updates", false, "send calendar invitation/update emails")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err = applySharedFlagDefaults(cfg)
	if err != nil {
		return err
	}
	if *title == "" {
		return errors.New("--title is required")
	}
	members := resolveMembers(cfg)
	if len(members) == 0 {
		return errors.New("members are required in config or --members")
	}
	if *durationMinutes <= 0 {
		return errors.New("--duration must be greater than 0")
	}
	if *stepMinutes <= 0 {
		return errors.New("--step must be greater than 0")
	}
	if *count < 0 {
		return errors.New("--count must be greater than or equal to 0")
	}
	selectionStrategy, err := parseSelectionStrategy(*strategy)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return err
	}

	service, err := googlecal.NewService(context.Background(), cfg.Credentials, cfg.Token, cfg.CalendarID)
	if err != nil {
		return err
	}
	slots, err := resolveHoldSlots(
		context.Background(),
		service,
		members,
		slotValues,
		ranges,
		stdin,
		loc,
		findOptions{
			Ranges:          ranges,
			DurationMinutes: *durationMinutes,
			StepMinutes:     *stepMinutes,
			Count:           *count,
			Strategy:        selectionStrategy,
			Debug:           *debugOutput,
		},
		stderr,
	)
	if err != nil {
		return err
	}
	if len(slots) == 0 {
		return errors.New("no hold slots resolved")
	}
	if err := service.CreateHolds(context.Background(), *title, slots, members, cfg.Timezone, *sendUpdates); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created %d hold events\n", len(slots))
	for _, slot := range slots {
		fmt.Fprintln(stdout, timefmt.FormatCustomerSlot(slot.In(loc)))
	}
	return nil
}

func runConfirm(args []string, stdout, stderr io.Writer) error {
	cfg, err := loadConfigForCommand(args, stderr)
	if err != nil {
		return err
	}

	fs := newFlagSet("schedule confirm", stderr)
	configureSharedFlags(fs, &cfg)
	title := fs.String("title", "", "case title")
	keepValue := fs.String("keep", "", "slot to keep: YYYY-MM-DD HH:MM")
	sendUpdates := fs.Bool("send-updates", false, "send calendar deletion/update emails")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err = applySharedFlagDefaults(cfg)
	if err != nil {
		return err
	}
	if *title == "" {
		return errors.New("--title is required")
	}
	if *keepValue == "" {
		return errors.New("--keep is required")
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return err
	}
	keep, err := timefmt.ParseKeep(*keepValue, loc)
	if err != nil {
		return err
	}

	service, err := googlecal.NewService(context.Background(), cfg.Credentials, cfg.Token, cfg.CalendarID)
	if err != nil {
		return err
	}
	deleted, err := service.Confirm(context.Background(), *title, keep, *sendUpdates)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "deleted %d hold events\n", deleted)
	return nil
}

func loadConfigForCommand(args []string, stderr io.Writer) (config.Config, error) {
	defaultPath, err := config.DefaultConfigPath()
	if err != nil {
		return config.Config{}, err
	}
	configPath := findConfigPath(args, defaultPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return config.Config{}, err
	}
	return cfg.WithDefaults()
}

func findConfigPath(args []string, defaultPath string) string {
	for i, arg := range args {
		if arg == "--config" && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "--config=") {
			return strings.TrimPrefix(arg, "--config=")
		}
	}
	return defaultPath
}

func configureSharedFlags(fs *flag.FlagSet, cfg *config.Config) {
	membersFlagValue = ""
	fs.StringVar(&cfg.Timezone, "timezone", cfg.Timezone, "IANA timezone")
	fs.StringVar(&cfg.CalendarID, "calendar", cfg.CalendarID, "calendar ID to create/delete holds")
	fs.StringVar(&cfg.Credentials, "credentials", cfg.Credentials, "OAuth client credentials JSON path")
	fs.StringVar(&cfg.Token, "token", cfg.Token, "OAuth token JSON path")
	fs.StringVar(&membersFlagValue, "members", "", "comma-separated member calendar IDs")
	fs.String("config", "", "config JSON path")
}

var membersFlagValue string

func applySharedFlagDefaults(cfg config.Config) (config.Config, error) {
	out, err := cfg.WithDefaults()
	if err != nil {
		return config.Config{}, err
	}
	if membersFlagValue != "" {
		out.Members = splitCommaList(membersFlagValue)
		membersFlagValue = ""
	}
	return out, nil
}

func resolveMembers(cfg config.Config) []string {
	members := slices.Clone(cfg.Members)
	for i := range members {
		members[i] = strings.TrimSpace(members[i])
	}
	return slices.DeleteFunc(members, func(member string) bool {
		return member == ""
	})
}

func parseWindows(values []string, loc *time.Location) ([]slotter.Interval, error) {
	windows := make([]slotter.Interval, 0, len(values))
	for _, value := range values {
		window, err := timefmt.ParseWindow(value, loc)
		if err != nil {
			return nil, err
		}
		windows = append(windows, window)
	}
	return windows, nil
}

func windowBounds(windows []slotter.Interval) (time.Time, time.Time) {
	minValue := windows[0].Start
	maxValue := windows[0].End
	for _, window := range windows[1:] {
		if window.Start.Before(minValue) {
			minValue = window.Start
		}
		if window.End.After(maxValue) {
			maxValue = window.End
		}
	}
	return minValue, maxValue
}

func findSlots(ctx context.Context, service *googlecal.Service, members []string, loc *time.Location, options findOptions, stderr io.Writer) ([]slotter.Interval, error) {
	windows, err := parseWindows(options.Ranges, loc)
	if err != nil {
		return nil, err
	}
	timeMin, timeMax := windowBounds(windows)
	busyByMember, err := service.BusyByMember(ctx, members, timeMin, timeMax)
	if err != nil {
		return nil, err
	}
	busy := flattenBusy(members, busyByMember)
	if options.Debug {
		writeFindDebug(stderr, members, windows, busyByMember, busy, loc)
	}

	candidates := slotter.CandidateSlots(
		windows,
		busy,
		time.Duration(options.DurationMinutes)*time.Minute,
		time.Duration(options.StepMinutes)*time.Minute,
		0,
	)
	return slotter.SelectSlots(candidates, options.Count, options.Strategy, loc), nil
}

func resolveHoldSlots(ctx context.Context, service *googlecal.Service, members []string, slotValues, ranges []string, stdin io.Reader, loc *time.Location, options findOptions, stderr io.Writer) ([]slotter.Interval, error) {
	if len(slotValues) > 0 {
		return parseWindows(slotValues, loc)
	}
	if len(ranges) > 0 {
		return findSlots(ctx, service, members, loc, options, stderr)
	}
	stdinSlots, err := parseHoldSlots(nil, stdin, loc)
	if err != nil {
		return nil, err
	}
	if len(stdinSlots) > 0 {
		return stdinSlots, nil
	}
	return nil, errors.New("at least one --slot, --range, or JSON stdin slot is required")
}

func parseHoldSlots(values []string, stdin io.Reader, loc *time.Location) ([]slotter.Interval, error) {
	if len(values) > 0 {
		return parseWindows(values, loc)
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}

	var payload findJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse slots JSON: %w", err)
	}

	slots := make([]slotter.Interval, 0, len(payload.Slots))
	for _, value := range payload.Slots {
		start, err := time.Parse(time.RFC3339Nano, value.Start)
		if err != nil {
			return nil, fmt.Errorf("parse slot start %q: %w", value.Start, err)
		}
		end, err := time.Parse(time.RFC3339Nano, value.End)
		if err != nil {
			return nil, fmt.Errorf("parse slot end %q: %w", value.End, err)
		}
		slots = append(slots, slotter.Interval{Start: start, End: end})
	}
	return slots, nil
}

func writeSlotsJSON(stdout io.Writer, slots []slotter.Interval) error {
	payload := findJSON{Slots: make([]slotJSON, 0, len(slots))}
	for _, slot := range slots {
		payload.Slots = append(payload.Slots, slotJSON{
			Start: slot.Start.Format(time.RFC3339),
			End:   slot.End.Format(time.RFC3339),
			Text:  timefmt.FormatCustomerSlot(slot),
		})
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func parseSelectionStrategy(value string) (slotter.SelectionStrategy, error) {
	switch slotter.SelectionStrategy(value) {
	case slotter.SelectionBalanced:
		return slotter.SelectionBalanced, nil
	case slotter.SelectionEarly:
		return slotter.SelectionEarly, nil
	default:
		return "", fmt.Errorf("unknown --strategy %q", value)
	}
}

func flattenBusy(members []string, busyByMember map[string][]slotter.Interval) []slotter.Interval {
	var busy []slotter.Interval
	for _, member := range members {
		busy = append(busy, busyByMember[member]...)
	}
	return busy
}

func writeFindDebug(stderr io.Writer, members []string, windows []slotter.Interval, busyByMember map[string][]slotter.Interval, busy []slotter.Interval, loc *time.Location) {
	available := slotter.AvailableIntervals(windows, busy)
	mergedBusy := slotter.Merge(busy)

	fmt.Fprintf(stderr, "debug: members=%d\n", len(members))
	for _, member := range members {
		fmt.Fprintf(stderr, "debug: member %s\n", member)
		for _, interval := range busyByMember[member] {
			fmt.Fprintf(stderr, "debug: member_busy %s %s\n", member, formatDebugInterval(interval, loc))
		}
	}
	fmt.Fprintf(stderr, "debug: windows=%d\n", len(windows))
	for _, window := range windows {
		fmt.Fprintf(stderr, "debug: window %s\n", formatDebugInterval(window, loc))
	}
	fmt.Fprintf(stderr, "debug: merged_busy=%d\n", len(mergedBusy))
	for _, interval := range mergedBusy {
		fmt.Fprintf(stderr, "debug: busy %s\n", formatDebugInterval(interval, loc))
	}
	fmt.Fprintf(stderr, "debug: available=%d\n", len(available))
	for _, interval := range available {
		fmt.Fprintf(stderr, "debug: available %s\n", formatDebugInterval(interval, loc))
	}
}

func formatDebugInterval(interval slotter.Interval, loc *time.Location) string {
	start := interval.Start.In(loc).Format("2006-01-02 15:04")
	end := interval.End.In(loc).Format("2006-01-02 15:04")
	return start + "-" + end
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: schedule <auth|find|hold|confirm> [options]")
}
