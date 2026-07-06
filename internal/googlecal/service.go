package googlecal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kokoichi206/cal-slotter/internal/slotter"
)

const calendarAPIBase = "https://www.googleapis.com/calendar/v3"

// Service wraps Google Calendar operations used by the CLI.
type Service struct {
	calendarID string
	client     *http.Client
}

type freeBusyRequest struct {
	TimeMin string                `json:"timeMin"`
	TimeMax string                `json:"timeMax"`
	Items   []freeBusyRequestItem `json:"items"`
}

type freeBusyRequestItem struct {
	ID string `json:"id"`
}

type freeBusyResponse struct {
	Calendars map[string]freeBusyCalendar `json:"calendars"`
}

type freeBusyCalendar struct {
	Busy   []timePeriod    `json:"busy"`
	Errors []calendarError `json:"errors"`
}

type timePeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type calendarError struct {
	Domain string `json:"domain"`
	Reason string `json:"reason"`
}

type eventDateTime struct {
	DateTime string `json:"dateTime,omitempty"`
	TimeZone string `json:"timeZone,omitempty"`
}

type eventAttendee struct {
	Email string `json:"email"`
}

type event struct {
	ID           string          `json:"id,omitempty"`
	Summary      string          `json:"summary,omitempty"`
	Attendees    []eventAttendee `json:"attendees,omitempty"`
	Transparency string          `json:"transparency,omitempty"`
	Start        *eventDateTime  `json:"start,omitempty"`
	End          *eventDateTime  `json:"end,omitempty"`
}

type eventsListResponse struct {
	Items         []event `json:"items"`
	NextPageToken string  `json:"nextPageToken"`
}

// HoldTitle formats the shared title used for temporary hold events.
func HoldTitle(title string) string {
	return "【仮押さえ】" + title + " 日程候補"
}

// Busy returns the union source intervals from all member calendars.
func (s *Service) Busy(ctx context.Context, members []string, timeMin, timeMax time.Time) ([]slotter.Interval, error) {
	byMember, err := s.BusyByMember(ctx, members, timeMin, timeMax)
	if err != nil {
		return nil, err
	}

	var intervals []slotter.Interval
	for _, member := range members {
		intervals = append(intervals, byMember[member]...)
	}
	return intervals, nil
}

// BusyByMember returns busy intervals keyed by member calendar ID.
func (s *Service) BusyByMember(ctx context.Context, members []string, timeMin, timeMax time.Time) (map[string][]slotter.Interval, error) {
	items := make([]freeBusyRequestItem, 0, len(members))
	for _, member := range members {
		items = append(items, freeBusyRequestItem{ID: member})
	}

	var resp freeBusyResponse
	if err := s.doJSON(ctx, http.MethodPost, calendarAPIBase+"/freeBusy", nil, freeBusyRequest{
		TimeMin: timeMin.Format(time.RFC3339),
		TimeMax: timeMax.Format(time.RFC3339),
		Items:   items,
	}, &resp); err != nil {
		return nil, err
	}

	byMember := make(map[string][]slotter.Interval, len(members))
	for _, member := range members {
		result, ok := resp.Calendars[member]
		if !ok {
			return nil, fmt.Errorf("freebusy response missing calendar %s", member)
		}
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("freebusy error for %s: %s", member, formatCalendarErrors(result.Errors))
		}
		for _, busy := range result.Busy {
			start, err := time.Parse(time.RFC3339Nano, busy.Start)
			if err != nil {
				return nil, fmt.Errorf("parse busy start for %s: %w", member, err)
			}
			end, err := time.Parse(time.RFC3339Nano, busy.End)
			if err != nil {
				return nil, fmt.Errorf("parse busy end for %s: %w", member, err)
			}
			byMember[member] = append(byMember[member], slotter.Interval{Start: start, End: end})
		}
	}

	return byMember, nil
}

// CreateHolds creates temporary hold events for the given slots.
func (s *Service) CreateHolds(ctx context.Context, title string, slots []slotter.Interval, members []string, timezone string, sendUpdates bool) error {
	attendees := make([]eventAttendee, 0, len(members))
	for _, member := range members {
		attendees = append(attendees, eventAttendee{Email: member})
	}

	for _, slot := range slots {
		body := event{
			Summary:      HoldTitle(title),
			Attendees:    attendees,
			Transparency: "opaque",
			Start: &eventDateTime{
				DateTime: slot.Start.Format(time.RFC3339),
				TimeZone: timezone,
			},
			End: &eventDateTime{
				DateTime: slot.End.Format(time.RFC3339),
				TimeZone: timezone,
			},
		}

		endpoint := calendarAPIBase + "/calendars/" + url.PathEscape(s.calendarID) + "/events"
		query := url.Values{"sendUpdates": []string{sendUpdatesValue(sendUpdates)}}
		if err := s.doJSON(ctx, http.MethodPost, endpoint, query, body, nil); err != nil {
			return fmt.Errorf("create hold %s: %w", slot.Start.Format(time.RFC3339), err)
		}
	}

	return nil
}

// Confirm deletes hold events with the same title except the slot to keep.
func (s *Service) Confirm(ctx context.Context, title string, keep time.Time, sendUpdates bool) (int, error) {
	holdTitle := HoldTitle(title)
	events, err := s.listFutureHolds(ctx, holdTitle)
	if err != nil {
		return 0, err
	}

	keepFound := false
	for _, event := range events {
		start, err := eventStart(event)
		if err != nil {
			return 0, err
		}
		if sameMinute(start, keep) {
			keepFound = true
			break
		}
	}
	if !keepFound {
		return 0, fmt.Errorf("keep slot not found for %q at %s", holdTitle, keep.Format(time.RFC3339))
	}

	deleted := 0
	for _, event := range events {
		start, err := eventStart(event)
		if err != nil {
			return 0, err
		}
		if sameMinute(start, keep) {
			continue
		}
		endpoint := calendarAPIBase + "/calendars/" + url.PathEscape(s.calendarID) + "/events/" + url.PathEscape(event.ID)
		query := url.Values{"sendUpdates": []string{sendUpdatesValue(sendUpdates)}}
		if err := s.doJSON(ctx, http.MethodDelete, endpoint, query, nil, nil); err != nil {
			return deleted, fmt.Errorf("delete hold %s: %w", event.ID, err)
		}
		deleted++
	}

	return deleted, nil
}

func sendUpdatesValue(sendUpdates bool) string {
	if sendUpdates {
		return "all"
	}
	return "none"
}

func (s *Service) listFutureHolds(ctx context.Context, holdTitle string) ([]event, error) {
	endpoint := calendarAPIBase + "/calendars/" + url.PathEscape(s.calendarID) + "/events"
	query := url.Values{
		"q":            []string{holdTitle},
		"timeMin":      []string{time.Now().Format(time.RFC3339)},
		"singleEvents": []string{"true"},
		"orderBy":      []string{"startTime"},
	}

	var matched []event
	for {
		var resp eventsListResponse
		if err := s.doJSON(ctx, http.MethodGet, endpoint, query, nil, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Items {
			if item.Summary == holdTitle {
				matched = append(matched, item)
			}
		}
		if resp.NextPageToken == "" {
			return matched, nil
		}
		query.Set("pageToken", resp.NextPageToken)
	}
}

func (s *Service) doJSON(ctx context.Context, method, endpoint string, query url.Values, body any, out any) error {
	var requestBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		requestBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return err
	}
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("%s %s: %s: %s", method, req.URL.String(), resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func eventStart(event event) (time.Time, error) {
	if event.Start == nil || event.Start.DateTime == "" {
		return time.Time{}, fmt.Errorf("event %s has no dateTime start", event.ID)
	}
	start, err := time.Parse(time.RFC3339Nano, event.Start.DateTime)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse event start %s: %w", event.ID, err)
	}
	return start, nil
}

func sameMinute(a, b time.Time) bool {
	return a.Truncate(time.Minute).Equal(b.Truncate(time.Minute))
}

func formatCalendarErrors(errors []calendarError) string {
	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		parts = append(parts, err.Domain+"/"+err.Reason)
	}
	return strings.Join(parts, ", ")
}
