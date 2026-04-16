package ffmpeg

import "testing"

// TestParseTrackList_CyrillicTitles verifies track title extraction with
// non-ASCII titles and full metadata blocks (as produced by mkvmerge).
func TestParseTrackList_CyrillicTitles(t *testing.T) {
	stderr := `  Stream #0:8(rus): Subtitle: ass (default)
    Metadata:
      title           : Надписи (Crunchyroll)
      BPS             : 44
      DURATION        : 00:21:02.240000000
  Stream #0:9(rus): Subtitle: ass
    Metadata:
      title           : Надписи (AniLibria)
      BPS             : 289
  Stream #0:10(rus): Subtitle: ass
    Metadata:
      title           : Полные (Crunchyroll)
  Stream #0:11(rus): Subtitle: ass
    Metadata:
      title           : Полные (CafeSubs)
  Stream #0:12(jpn): Subtitle: hdmv_pgs_subtitle, 1920x1080
`

	tracks := parseTrackList(stderr)

	if len(tracks) != 4 {
		t.Fatalf("expected 4 ASS tracks, got %d: %+v", len(tracks), tracks)
	}

	expected := []string{
		"Надписи (Crunchyroll)",
		"Надписи (AniLibria)",
		"Полные (Crunchyroll)",
		"Полные (CafeSubs)",
	}

	for i, exp := range expected {
		if tracks[i].Title != exp {
			t.Errorf("track[%d].Title = %q, want %q", i, tracks[i].Title, exp)
		}
		if tracks[i].Language != "rus" {
			t.Errorf("track[%d].Language = %q, want rus", i, tracks[i].Language)
		}
	}
}

// TestParseTrackList_WindowsLineEndings verifies parsing with CRLF line endings.
func TestParseTrackList_WindowsLineEndings(t *testing.T) {
	stderr := "  Stream #0:2(rus): Subtitle: ass\r\n    Metadata:\r\n      title           : Russian [Anku]\r\n"
	tracks := parseTrackList(stderr)

	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].Title != "Russian [Anku]" {
		t.Errorf("title = %q, want 'Russian [Anku]'", tracks[0].Title)
	}
}
