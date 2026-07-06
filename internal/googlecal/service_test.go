package googlecal

import "testing"

func TestMeetingCode(t *testing.T) {
	event := event{
		ConferenceData: &eventConferenceData{
			ConferenceID: "abc-defg-hij",
		},
		HangoutLink: "https://meet.google.com/unused-code",
	}

	got := meetingCode(event)
	if got != "abc-defg-hij" {
		t.Fatalf("meetingCode() = %q, want %q", got, "abc-defg-hij")
	}
}

func TestMeetingCodeFromVideoEntryPoint(t *testing.T) {
	event := event{
		ConferenceData: &eventConferenceData{
			EntryPoints: []conferenceEntryPoint{
				{EntryPointType: "phone", URI: "tel:+10000000000"},
				{EntryPointType: "video", URI: "https://meet.google.com/mot-tqmj-ceg"},
			},
		},
	}

	got := meetingCode(event)
	if got != "mot-tqmj-ceg" {
		t.Fatalf("meetingCode() = %q, want %q", got, "mot-tqmj-ceg")
	}
}

func TestMeetingCodeFromURLRejectsNonMeetURL(t *testing.T) {
	got := meetingCodeFromURL("https://example.com/mot-tqmj-ceg")
	if got != "" {
		t.Fatalf("meetingCodeFromURL() = %q, want empty string", got)
	}
}
