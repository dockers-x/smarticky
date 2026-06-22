package notes

import "testing"

func TestParseWikiLinksAggregatesByNormalizedTarget(t *testing.T) {
	got := ParseWikiLinks("See [[Project Alpha]] then [[ project alpha | Alpha ]] and [[Other]].")

	if len(got) != 2 {
		t.Fatalf("expected 2 links, got %d: %+v", len(got), got)
	}
	if got[0].TargetRef != "Project Alpha" || got[0].TargetRefNorm != "project alpha" || got[0].DisplayText != "Project Alpha" || got[0].OccurrenceCount != 2 {
		t.Fatalf("unexpected first link: %+v", got[0])
	}
	if got[1].TargetRef != "Other" || got[1].TargetRefNorm != "other" || got[1].DisplayText != "Other" || got[1].OccurrenceCount != 1 {
		t.Fatalf("unexpected second link: %+v", got[1])
	}
}

func TestParseWikiLinksIgnoresEmptyTargets(t *testing.T) {
	got := ParseWikiLinks("[[]] [[ | Alias ]] [[Valid|Shown]]")

	if len(got) != 1 {
		t.Fatalf("expected 1 link, got %d: %+v", len(got), got)
	}
	if got[0].TargetRef != "Valid" || got[0].DisplayText != "Shown" {
		t.Fatalf("unexpected link: %+v", got[0])
	}
}
