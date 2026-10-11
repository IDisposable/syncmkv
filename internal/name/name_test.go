package name

import "testing"

func TestMovieTitle(t *testing.T) {
	f := FromGroups([]Group{{
		Items: []Item{{Name: "TITLE", Value: "Flyboys"}},
	}})
	title, kind, _, ok := Desired(f, false)
	if !ok || kind != "movie" || title != "Flyboys" {
		t.Fatalf("got %q %s ok=%v", title, kind, ok)
	}
}

func TestMovieShowIsNotASeries(t *testing.T) {
	f := FromGroups([]Group{{
		Items: []Item{
			{Name: "SHOW", Value: "Hart's War"},
			{Name: "TITLE", Value: "Hart's War"},
		},
	}})
	title, kind, _, ok := Desired(f, false)
	if !ok || kind != "movie" || title != "Hart's War" {
		t.Fatalf("got %q %s ok=%v", title, kind, ok)
	}
}

func TestSeriesGroups(t *testing.T) {
	f := FromGroups([]Group{
		{Items: []Item{{Name: "TITLE", Value: "Andromeda"}}},
		{Items: []Item{{Name: "PART_NUMBER", Value: "1"}}},
		{Items: []Item{
			{Name: "SHOW", Value: "Andromeda"},
			{Name: "SEASON_NUMBER", Value: "1"},
			{Name: "EPISODE_ID", Value: "1"},
			{Name: "TITLE", Value: "Under the Night (1)"},
			{Name: "PART_NUMBER", Value: "1"},
		}},
	})
	title, kind, _, ok := Desired(f, false)
	if !ok || kind != "series" || title != "Under the Night (1)" {
		t.Fatalf("title only %q %s ok=%v", title, kind, ok)
	}
	title, kind, _, ok = Desired(f, true)
	want := "Andromeda - S01E01 - Under the Night (1)"
	if !ok || kind != "series" || title != want {
		t.Fatalf("got %q %s ok=%v", title, kind, ok)
	}
}

func TestPathFillsMissingNumbers(t *testing.T) {
	tags := FromGroups([]Group{
		{Items: []Item{{Name: "TITLE", Value: "Andromeda"}}},
		{Items: []Item{{Name: "TITLE", Value: "Under the Night"}}},
	})
	// The second group has only a title, so the numbers come from the path.
	tags.SeriesHint = true
	tags.Series = "Andromeda"
	tags.EpisodeTitle = "Under the Night"
	path := FromPath("/mnt/zulu/tv/andromeda/Season 01/Andromeda - S01E01 - Under the Night.mkv")
	got := Merge(tags, path)
	title, _, _, ok := Desired(got, true)
	if !ok || title != "Andromeda - S01E01 - Under the Night" {
		t.Fatalf("got %q ok=%v fields=%+v", title, ok, got)
	}
}

func TestEpisodePath(t *testing.T) {
	f := FromPath("/tv/andromeda/Season 01/Andromeda - S01E01 - Under the Night.mkv")
	title, kind, _, ok := Desired(f, true)
	if !ok || kind != "series" || title != "Andromeda - S01E01 - Under the Night" {
		t.Fatalf("got %q %s ok=%v fields=%+v", title, kind, ok, f)
	}
}

func TestEpisodePartStays(t *testing.T) {
	f := FromGroups([]Group{
		{Items: []Item{{Name: "TITLE", Value: "Andromeda"}}},
		{Items: []Item{{Name: "PART_NUMBER", Value: "5"}}},
		{Items: []Item{
			{Name: "SEASON_NUMBER", Value: "5"},
			{Name: "EPISODE_ID", Value: "1"},
			{Name: "TITLE", Value: "The Weight (1)"},
		}},
	})
	title, _, _, ok := Desired(f, false)
	if !ok || title != "The Weight (1)" {
		t.Fatalf("title only %q ok=%v", title, ok)
	}
	title, _, _, ok = Desired(f, true)
	if !ok || title != "Andromeda - S05E01 - The Weight (1)" {
		t.Fatalf("series title %q ok=%v", title, ok)
	}
	both := FromGroups([]Group{{
		Items: []Item{{Name: "TITLE", Value: "The Weight (1) (2005)"}},
	}})
	title, _, _, ok = Desired(both, false)
	if !ok || title != "The Weight (1)" {
		t.Fatalf("year after part %q ok=%v", title, ok)
	}
}

func TestMoviePath(t *testing.T) {
	f := FromPath("/movies/war/Flyboys (2006).mkv")
	title, kind, _, ok := Desired(f, false)
	if !ok || kind != "movie" || title != "Flyboys" {
		t.Fatalf("got %q %s ok=%v", title, kind, ok)
	}
}

func TestMovieYearRemoved(t *testing.T) {
	f := FromGroups([]Group{{
		Items: []Item{{Name: "TITLE", Value: "A View to a Kill (1985)"}},
	}})
	title, kind, _, ok := Desired(f, false)
	if !ok || kind != "movie" || title != "A View to a Kill" {
		t.Fatalf("tag title %q %s ok=%v", title, kind, ok)
	}
	path := FromPath("/movies/Action/007/A View to a Kill (1985).mkv")
	title, kind, _, ok = Desired(path, false)
	if !ok || kind != "movie" || title != "A View to a Kill" {
		t.Fatalf("path title %q %s ok=%v", title, kind, ok)
	}
	kept := FromGroups([]Group{{
		Items: []Item{{Name: "TITLE", Value: "1917"}},
	}})
	title, _, _, ok = Desired(kept, false)
	if !ok || title != "1917" {
		t.Fatalf("kept %q ok=%v", title, ok)
	}
}

func TestReleaseName(t *testing.T) {
	f := FromPath("/tv/andromeda/Season 02/Andromeda.S02E01.The.Widening.Gyre.1080p.WEB.x264-GROUP.mkv")
	title, kind, _, ok := Desired(f, true)
	if !ok || kind != "series" || title != "Andromeda - S02E01 - The Widening Gyre" {
		t.Fatalf("got %q %s ok=%v fields=%+v", title, kind, ok, f)
	}
}
