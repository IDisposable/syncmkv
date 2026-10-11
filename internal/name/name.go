// Package name builds a segment title from Matroska tag groups or from a file path.
//
// A series file has separate tag groups. One group holds the show title.
// One group holds the season number. One group holds the episode.
// A movie file has one group, and its TITLE is the movie title.
// SHOW on that movie group is the same title. It does not make the file a series.
package name

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Group is one Matroska tag target and the names stored on it.
type Group struct {
	Items []Item
}

// Item is one tag name and its text.
type Item struct {
	Name  string
	Value string
}

// Fields is the movie or episode identity.
type Fields struct {
	Series       string
	Season       int
	SeasonSet    bool
	Episode      int
	EpisodeSet   bool
	EpisodeEnd   int
	EpisodeTitle string
	MovieTitle   string
	SeriesHint   bool
}

var (
	reSE = regexp.MustCompile(`(?i)^(.*?)[ \t._-]*[Ss](\d{1,3})[ \t._-]*[Ee](\d{1,4})(?:[ \t._-]*[Ee](\d{1,4}))?(?:[ \t._-]+(.*))?$`)
	reX  = regexp.MustCompile(`(?i)^(.+?)[ \t._-]+(\d{1,2})x(\d{1,3})(?:[ \t._-]+(.*))?$`)

	reSeasonDir = regexp.MustCompile(`(?i)^season[ \t._-]*(\d{1,3})$`)
	reSpecials  = regexp.MustCompile(`(?i)^specials?$`)
	reTrailYear = regexp.MustCompile(`\s*\(((?:19|20)\d{2})\)$`)
	reBracket   = regexp.MustCompile(`\[[^\]]*\]|\{[^}]*\}`)
	reParen     = regexp.MustCompile(`\(([^)]*)\)`)
	reGroup     = regexp.MustCompile(`^-[A-Za-z0-9]{2,}$`)

	reLeadEp = regexp.MustCompile(`(?i)^(?:episode[ \t._-]+|ep[ \t._-]*|e)(\d{1,4})(?:[ \t._-]+(.*))?$`)
	reNum    = regexp.MustCompile(`(?i)^(\d{1,4})(?:[ \t._-]+(.*))?$`)
	reMid    = regexp.MustCompile(`(?i)^(.+?)[ \t._-]+(?:e)?(\d{1,3})[ \t._-]+(.+)$`)
)

var junkSet = map[string]bool{
	"1080p": true, "720p": true, "2160p": true, "480p": true, "576p": true,
	"4k": true, "8k": true, "uhd": true, "hdr": true, "hdr10": true, "hdr10plus": true,
	"dv": true, "dolbyvision": true,
	"bluray": true, "blu-ray": true, "bdrip": true, "brrip": true,
	"webrip": true, "web-dl": true, "webdl": true, "web": true, "hdtv": true,
	"dvdrip": true, "dvd": true, "remux": true,
	"x264": true, "x265": true, "h264": true, "h265": true, "hevc": true,
	"avc": true, "av1": true, "xvid": true, "divx": true,
	"10bit": true, "8bit": true, "hi10p": true,
	"aac": true, "ac3": true, "eac3": true, "dts": true, "dtshd": true, "dts-hd": true,
	"truehd": true, "flac": true, "atmos": true, "opus": true,
	"proper": true, "repack": true, "rerip": true, "extended": true, "unrated": true,
	"theatrical": true, "remastered": true, "limited": true, "internal": true,
	"amzn": true, "nf": true, "dsnp": true, "hmax": true, "atvp": true, "hulu": true, "pcok": true,
	"ddp5.1": true, "ddp5": true, "dd5.1": true, "5.1": true, "7.1": true, "2.0": true,
}

var genericDir = map[string]bool{
	"video": true, "videos": true, "movie": true, "movies": true,
	"film": true, "films": true, "tv": true, "series": true, "shows": true, "show": true,
	"mkv": true, "downloads": true, "download": true, "media": true,
	"extra": true, "extras": true, "subs": true, "subtitles": true, "subtitle": true,
}

// FromGroups reads the movie or episode identity from Matroska tag groups.
func FromGroups(groups []Group) Fields {
	var f Fields
	var show, bareTitle, movieTitle, episodeTitle string
	for _, g := range groups {
		titles, parts, groupShow, seasonN, seasonOK, episodeN, episodeOK := flatten(g)
		if seasonOK {
			f.SeriesHint = true
			if !f.SeasonSet {
				f.Season, f.SeasonSet = seasonN, true
			}
		}
		if episodeOK {
			f.SeriesHint = true
			if !f.EpisodeSet {
				f.Episode, f.EpisodeSet = episodeN, true
			}
		}
		if groupShow != "" {
			show = groupShow
		}
		onlyPart := len(titles) == 0 && groupShow == "" && !seasonOK && !episodeOK && len(parts) == 1
		if onlyPart && !f.SeasonSet {
			f.Season, f.SeasonSet = parts[0], true
			f.SeriesHint = true
		}
		detail := len(parts) > 0 || seasonOK || episodeOK || groupShow != ""
		if len(titles) > 0 && !detail {
			if bareTitle == "" {
				bareTitle = titles[0]
			}
		}
		if len(titles) > 0 && detail {
			if episodeTitle == "" {
				episodeTitle = titles[0]
			}
			if movieTitle == "" {
				movieTitle = titles[0]
			}
			if !f.EpisodeSet && len(parts) > 0 && (episodeOK || seasonOK || groupShow != "") {
				f.Episode, f.EpisodeSet = parts[0], true
			}
		}
	}
	if bareTitle != "" && f.SeriesHint {
		f.Series = bareTitle
	}
	if f.Series == "" && show != "" && f.SeriesHint {
		f.Series = show
	}
	if f.SeriesHint {
		f.EpisodeTitle = episodeTitle
		if strings.EqualFold(strings.TrimSpace(f.EpisodeTitle), f.Series) {
			f.EpisodeTitle = ""
		}
		return f
	}
	if movieTitle != "" {
		f.MovieTitle = movieTitle
		return f
	}
	f.MovieTitle = bareTitle
	return f
}

func flatten(g Group) (titles []string, parts []int, show string, seasonN int, seasonOK bool, episodeN int, episodeOK bool) {
	var walk func(items []Item)
	walk = func(items []Item) {
		for _, it := range items {
			key := strings.ToUpper(strings.TrimSpace(it.Name))
			val := strings.TrimSpace(it.Value)
			switch key {
			case "TITLE":
				if val != "" {
					titles = append(titles, val)
				}
			case "SHOW", "SERIES", "SERIES_NAME", "SHOW_NAME", "TVSHOW":
				if show == "" && val != "" {
					show = val
				}
			case "SEASON", "SEASON_NUMBER":
				if n, ok := atoi(val); ok && !seasonOK {
					seasonN, seasonOK = n, true
				}
			case "EPISODE", "EPISODE_NUMBER", "EPISODE_ID":
				if n, ok := atoi(val); ok && !episodeOK {
					episodeN, episodeOK = n, true
				}
			case "PART_NUMBER":
				if n, ok := atoi(val); ok {
					parts = append(parts, n)
				}
			}
		}
	}
	walk(g.Items)
	return titles, parts, show, seasonN, seasonOK, episodeN, episodeOK
}

// FromPath reads the movie or episode identity from a directory and a file name.
func FromPath(path string) Fields {
	base := filepath.Base(path)
	stem := normalize(strings.TrimSuffix(base, filepath.Ext(base)))
	f := parseStem(stem)
	show, season, seasonOK, folder := fromDirs(path)
	if seasonOK {
		f.SeriesHint = true
		if !f.SeasonSet {
			f.Season, f.SeasonSet = season, true
		}
		if f.Series == "" {
			f.Series = show
		}
		if !f.EpisodeSet {
			if ep, title, ok := episodeInSeason(stem); ok {
				f.Episode, f.EpisodeSet = ep, true
				if f.EpisodeTitle == "" {
					f.EpisodeTitle = title
				}
			}
		}
	}
	if f.SeriesHint {
		f.Series = cleanRelease(trimSep(f.Series))
		f.EpisodeTitle = cleanRelease(trimSep(f.EpisodeTitle))
		return f
	}
	title := cleanRelease(stem)
	if title == "" || genericName(title) {
		title = folder
	}
	f.MovieTitle = withoutYear(title)
	return f
}

// Merge keeps tag values and fills empty fields from the path.
// A movie tag is not changed into a series by the path.
func Merge(tags, path Fields) Fields {
	empty := tags.Series == "" && !tags.SeasonSet && !tags.EpisodeSet && tags.MovieTitle == "" && tags.EpisodeTitle == "" && !tags.SeriesHint
	if empty {
		return path
	}
	if tags.SeriesHint {
		out := tags
		if out.Series == "" {
			out.Series = path.Series
		}
		if !out.SeasonSet && path.SeasonSet {
			out.Season, out.SeasonSet = path.Season, true
		}
		if !out.EpisodeSet && path.EpisodeSet {
			out.Episode, out.EpisodeSet = path.Episode, true
			out.EpisodeEnd = path.EpisodeEnd
		}
		if out.EpisodeTitle == "" {
			out.EpisodeTitle = path.EpisodeTitle
		}
		return out
	}
	if tags.MovieTitle == "" {
		tags.MovieTitle = path.MovieTitle
	}
	return tags
}

// Desired returns the segment title.
// withSeries selects "Series - S01E01 - Title". The default is the episode title.
func Desired(f Fields, withSeries bool) (title, kind, reason string, ok bool) {
	if f.SeriesHint {
		if withSeries {
			if f.Series == "" || !f.SeasonSet || !f.EpisodeSet {
				return "", "series", "series season or episode is missing", false
			}
			series := withoutYear(f.Series)
			code := fmt.Sprintf("S%02dE%02d", f.Season, f.Episode)
			if f.EpisodeEnd > f.Episode {
				code = fmt.Sprintf("S%02dE%02d-E%02d", f.Season, f.Episode, f.EpisodeEnd)
			}
			title = series + " - " + code
			ep := withoutYear(f.EpisodeTitle)
			if ep != "" && !strings.EqualFold(ep, series) {
				title += " - " + ep
			}
			return title, "series", "", true
		}
		ep := withoutYear(f.EpisodeTitle)
		if ep == "" {
			return "", "series", "episode title is missing", false
		}
		return ep, "series", "", true
	}
	if movie := withoutYear(f.MovieTitle); movie != "" {
		return movie, "movie", "", true
	}
	return "", "", "no title found", false
}

func parseStem(stem string) Fields {
	var f Fields
	if m := reSE.FindStringSubmatch(stem); m != nil {
		f.SeriesHint = true
		f.Series = trimSep(m[1])
		f.Season, f.SeasonSet = mustInt(m[2])
		f.Episode, f.EpisodeSet = mustInt(m[3])
		if m[4] != "" {
			end, _ := strconv.Atoi(m[4])
			if end > f.Episode {
				f.EpisodeEnd = end
			}
		}
		f.EpisodeTitle = trimSep(m[5])
		return f
	}
	if m := reX.FindStringSubmatch(stem); m != nil {
		f.SeriesHint = true
		f.Series = trimSep(m[1])
		f.Season, f.SeasonSet = mustInt(m[2])
		f.Episode, f.EpisodeSet = mustInt(m[3])
		f.EpisodeTitle = trimSep(m[4])
		return f
	}
	return f
}

func episodeInSeason(stem string) (int, string, bool) {
	if m := reLeadEp.FindStringSubmatch(stem); m != nil {
		ep, _ := strconv.Atoi(m[1])
		return ep, trimSep(m[2]), ep > 0
	}
	if m := reMid.FindStringSubmatch(stem); m != nil {
		ep, _ := strconv.Atoi(m[2])
		if ep == 0 || yearLike(ep) {
			return 0, "", false
		}
		return ep, trimSep(m[3]), true
	}
	if m := reNum.FindStringSubmatch(stem); m != nil {
		ep, _ := strconv.Atoi(m[1])
		if ep == 0 || yearLike(ep) {
			return 0, "", false
		}
		return ep, trimSep(m[2]), true
	}
	return 0, "", false
}

func fromDirs(path string) (show string, season int, seasonOK bool, folder string) {
	dir := filepath.Dir(path)
	var names []string
	for i := 0; i < 8; i++ {
		base := filepath.Base(dir)
		parent := filepath.Dir(dir)
		if base == "" || base == "." || parent == dir {
			break
		}
		names = append(names, base)
		dir = parent
	}
	for i, base := range names {
		if reSpecials.MatchString(base) {
			return nextShow(names[i+1:]), 0, true, cleanFolder(base)
		}
		if m := reSeasonDir.FindStringSubmatch(base); m != nil {
			n, _ := strconv.Atoi(m[1])
			return nextShow(names[i+1:]), n, true, cleanFolder(base)
		}
	}
	for _, base := range names {
		if skipDir(base) {
			continue
		}
		return "", 0, false, cleanFolder(base)
	}
	return "", 0, false, ""
}

func nextShow(names []string) string {
	for _, base := range names {
		if skipDir(base) || reSeasonDir.MatchString(base) || reSpecials.MatchString(base) {
			continue
		}
		return stripYear(cleanFolder(base))
	}
	return ""
}

func skipDir(base string) bool {
	low := strings.ToLower(cleanFolder(base))
	return genericDir[low] || junkToken(low)
}

func normalize(stem string) string {
	stem = strings.TrimSpace(stem)
	if strings.Contains(stem, ".") && !strings.Contains(stem, " ") {
		stem = strings.ReplaceAll(stem, ".", " ")
	}
	stem = strings.ReplaceAll(stem, "_", " ")
	return strings.Join(strings.Fields(stem), " ")
}

func cleanFolder(s string) string {
	return normalize(s)
}

func stripYear(s string) string {
	return strings.TrimSpace(reTrailYear.ReplaceAllString(s, ""))
}

// withoutYear removes one trailing parenthetical year, such as (1985).
// A part number such as (1) or (2) stays. A title that is only a year, such as 1917, stays.
func withoutYear(s string) string {
	s = strings.TrimSpace(s)
	next := stripYear(s)
	if next == "" {
		return s
	}
	return next
}

func trimSep(s string) string {
	return strings.Trim(s, " \t.-_")
}

func cleanRelease(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = reBracket.ReplaceAllString(s, " ")
	s = reParen.ReplaceAllStringFunc(s, func(m string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(m, "("), ")")
		if parenJunk(inner) {
			return " "
		}
		return m
	})
	fields := strings.Fields(s)
	for len(fields) > 0 && junkToken(fields[len(fields)-1]) {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

func parenJunk(inner string) bool {
	parts := strings.Fields(inner)
	if len(parts) == 0 {
		return true
	}
	for _, p := range parts {
		if !junkToken(p) {
			return false
		}
	}
	return true
}

func junkToken(tok string) bool {
	t := strings.ToLower(strings.Trim(tok, ".-_"))
	if t == "" {
		return true
	}
	if junkSet[t] {
		return true
	}
	if strings.HasPrefix(tok, "-") && reGroup.MatchString(tok) {
		return true
	}
	if i := strings.IndexByte(t, '-'); i > 0 && junkSet[t[:i]] {
		return true
	}
	return false
}

func genericName(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "video", "movie", "film", "title", "stream", "track", "mkv":
		return true
	default:
		return false
	}
}

func yearLike(n int) bool {
	return n >= 1900 && n <= 2100
}

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

func mustInt(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
