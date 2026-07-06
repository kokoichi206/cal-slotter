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
const meetAPIBase = "https://meet.googleapis.com/v2"

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
	ID             string               `json:"id,omitempty"`
	Summary        string               `json:"summary,omitempty"`
	HangoutLink    string               `json:"hangoutLink,omitempty"`
	Attendees      []eventAttendee      `json:"attendees,omitempty"`
	Transparency   string               `json:"transparency,omitempty"`
	Start          *eventDateTime       `json:"start,omitempty"`
	End            *eventDateTime       `json:"end,omitempty"`
	ConferenceData *eventConferenceData `json:"conferenceData,omitempty"`
}

type eventConferenceData struct {
	CreateRequest *conferenceCreateRequest `json:"createRequest,omitempty"`
	ConferenceID  string                   `json:"conferenceId,omitempty"`
	EntryPoints   []conferenceEntryPoint   `json:"entryPoints,omitempty"`
}

type conferenceCreateRequest struct {
	RequestID             string                `json:"requestId"`
	ConferenceSolutionKey conferenceSolutionKey `json:"conferenceSolutionKey"`
}

type conferenceSolutionKey struct {
	Type string `json:"type"`
}

type conferenceEntryPoint struct {
	EntryPointType string `json:"entryPointType,omitempty"`
	URI            string `json:"uri,omitempty"`
}

type eventsListResponse struct {
	Items         []event `json:"items"`
	NextPageToken string  `json:"nextPageToken"`
}

type meetSpace struct {
	Name   string           `json:"name,omitempty"`
	Config *meetSpaceConfig `json:"config,omitempty"`
}

type meetSpaceConfig struct {
	ArtifactConfig *meetArtifactConfig `json:"artifactConfig,omitempty"`
}

type meetArtifactConfig struct {
	RecordingConfig     *meetRecordingConfig     `json:"recordingConfig,omitempty"`
	TranscriptionConfig *meetTranscriptionConfig `json:"transcriptionConfig,omitempty"`
	SmartNotesConfig    *meetSmartNotesConfig    `json:"smartNotesConfig,omitempty"`
}

type meetRecordingConfig struct {
	AutoRecordingGeneration string `json:"autoRecordingGeneration"`
}

type meetTranscriptionConfig struct {
	AutoTranscriptionGeneration string `json:"autoTranscriptionGeneration"`
}

type meetSmartNotesConfig struct {
	AutoSmartNotesGeneration string `json:"autoSmartNotesGeneration"`
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
func (s *Service) CreateHolds(ctx context.Context, title string, slots []slotter.Interval, members []string, timezone string, sendUpdates bool, meetArtifacts bool) error {
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
		if meetArtifacts {
			requestID, err := newConferenceRequestID()
			if err != nil {
				return fmt.Errorf("create conference request ID: %w", err)
			}
			body.ConferenceData = &eventConferenceData{
				CreateRequest: &conferenceCreateRequest{
					RequestID: requestID,
					ConferenceSolutionKey: conferenceSolutionKey{
						Type: "hangoutsMeet",
					},
				},
			}
		}

		endpoint := calendarAPIBase + "/calendars/" + url.PathEscape(s.calendarID) + "/events"
		query := url.Values{"sendUpdates": []string{sendUpdatesValue(sendUpdates)}}
		var created event
		var out any
		if meetArtifacts {
			query.Set("conferenceDataVersion", "1")
			out = &created
		}
		if err := s.doJSON(ctx, http.MethodPost, endpoint, query, body, out); err != nil {
			return fmt.Errorf("create hold %s: %w", slot.Start.Format(time.RFC3339), err)
		}
		if meetArtifacts {
			if err := s.enableAutoArtifacts(ctx, created); err != nil {
				return fmt.Errorf("enable Meet artifacts for hold %s: %w", slot.Start.Format(time.RFC3339), err)
			}
		}
	}

	return nil
}

func (s *Service) enableAutoArtifacts(ctx context.Context, calendarEvent event) error {
	meetingCode, err := s.meetingCodeForEvent(ctx, calendarEvent)
	if err != nil {
		return err
	}

	var space meetSpace
	if err := s.doJSON(ctx, http.MethodGet, meetAPIBase+"/spaces/"+url.PathEscape(meetingCode), nil, nil, &space); err != nil {
		return err
	}

	body := meetSpace{
		Config: &meetSpaceConfig{
			ArtifactConfig: &meetArtifactConfig{
				RecordingConfig: &meetRecordingConfig{
					AutoRecordingGeneration: "ON",
				},
				TranscriptionConfig: &meetTranscriptionConfig{
					AutoTranscriptionGeneration: "ON",
				},
				SmartNotesConfig: &meetSmartNotesConfig{
					AutoSmartNotesGeneration: "ON",
				},
			},
		},
	}

	return s.doJSON(ctx, http.MethodPatch, meetAPIBase+"/"+space.Name, nil, body, nil)
}

func (s *Service) meetingCodeForEvent(ctx context.Context, calendarEvent event) (string, error) {
	if code := meetingCode(calendarEvent); code != "" {
		return code, nil
	}

	endpoint := calendarAPIBase + "/calendars/" + url.PathEscape(s.calendarID) + "/events/" + url.PathEscape(calendarEvent.ID)
	query := url.Values{"conferenceDataVersion": []string{"1"}}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var latest event
		if err := s.doJSON(ctx, http.MethodGet, endpoint, query, nil, &latest); err != nil {
			return "", err
		}
		if code := meetingCode(latest); code != "" {
			return code, nil
		}
		if !time.Now().Before(deadline) {
			return "", fmt.Errorf("Meet conference data was not generated for event %s", calendarEvent.ID)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
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

func newConferenceRequestID() (string, error) {
	state, err := randomState()
	if err != nil {
		return "", err
	}
	return "slotter-" + state, nil
}

func meetingCode(event event) string {
	if event.ConferenceData != nil {
		if event.ConferenceData.ConferenceID != "" {
			return event.ConferenceData.ConferenceID
		}
		for _, entryPoint := range event.ConferenceData.EntryPoints {
			if entryPoint.EntryPointType == "video" {
				return meetingCodeFromURL(entryPoint.URI)
			}
		}
	}
	return meetingCodeFromURL(event.HangoutLink)
}

func meetingCodeFromURL(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if parsed.Host != "meet.google.com" {
		return ""
	}
	return strings.Trim(parsed.Path, "/")
}

func formatCalendarErrors(errors []calendarError) string {
	parts := make([]string, 0, len(errors))
	for _, err := range errors {
		parts = append(parts, err.Domain+"/"+err.Reason)
	}
	return strings.Join(parts, ", ")
}
