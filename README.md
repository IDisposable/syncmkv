# syncmkv

syncmkv sets the segment title inside each MKV file.
Players such as VLC show this title.
The program does not rename the file.

The title comes from the tags in the file.
MKV Tag Editor writes those tags.
A movie uses the movie title.
A TV episode uses the episode title.
Pass `-series` to use `Series - S01E01 - Episode Title` instead.

When a tag is missing, the program reads the folder name and the file name.

The program does not write a file until you pass `-apply`.
`-apply` updates the header in place.
Pass `-rewrite` as well when a longer title does not fit in the header.
Without `-rewrite`, that file is skipped.

## Build

The program needs Go 1.27.1.
It uses the `mkvgo` module to read and write Matroska files.

```
go build -o bin/syncmkv ./cmd/syncmkv
```

`make dist` builds Linux amd64 and Windows amd64.

## Use

```
syncmkv /path/to/videos
syncmkv -apply /path/to/videos
```

A directory includes MKV files in its subdirectories.
The top directory does not need to contain an MKV file.
No path means the current directory.

## Title text

Movie: `Flyboys`

The year in parentheses is left out. `A View to a Kill (1985)` becomes `A View to a Kill`.
A part number stays. `The Weight (1)` stays `The Weight (1)`.

TV episode: `Under the Night (1)`

With `-series`: `Andromeda - S01E01 - Under the Night (1)`

## Tags

The show title is a TITLE tag in its own tag group.
The season number is PART_NUMBER in its own tag group, or SEASON_NUMBER.
The episode number is EPISODE_ID, or PART_NUMBER on the episode group.
The episode name is TITLE on the episode group.
A movie stores its title in TITLE.
A movie can also store that same title in SHOW. That does not make it a series.
