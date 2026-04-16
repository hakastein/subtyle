package parser

import "time"

type Color struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
	A uint8 `json:"a"` // 255 = fully opaque, 0 = fully transparent
}

type SubtitleStyle struct {
	Name            string  `json:"name"`
	FontName        string  `json:"fontName"`
	FontSize        float64 `json:"fontSize"`
	Bold            bool    `json:"bold"`
	Italic          bool    `json:"italic"`
	Underline       bool    `json:"underline"`
	Strikeout       bool    `json:"strikeout"`
	PrimaryColour   Color   `json:"primaryColour"`
	SecondaryColour Color   `json:"secondaryColour"`
	OutlineColour   Color   `json:"outlineColour"`
	BackColour      Color   `json:"backColour"`
	Outline         float64 `json:"outline"`
	Shadow          float64 `json:"shadow"`
	ScaleX          float64 `json:"scaleX"`
	ScaleY          float64 `json:"scaleY"`
	Spacing         float64 `json:"spacing"`
	Angle           float64 `json:"angle"`
	Alignment       int     `json:"alignment"` // ASS numpad 1-9
	MarginL         int     `json:"marginL"`
	MarginR         int     `json:"marginR"`
	MarginV         int     `json:"marginV"`
}

type SubtitleEvent struct {
	StyleName string        `json:"styleName"`
	StartTime time.Duration `json:"startTime"`
	EndTime   time.Duration `json:"endTime"`
	Text      string        `json:"text"`
}

type SubtitleFile struct {
	ID         string          `json:"id"`
	Path       string          `json:"path"`
	Source     string          `json:"source"` // "external" or "embedded"
	TrackID    int             `json:"trackId"`
	TrackTitle string          `json:"trackTitle"` // display name for embedded tracks
	Styles     []SubtitleStyle `json:"styles"`
	Events     []SubtitleEvent `json:"events"`
}
