package mkv

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mkvparse "github.com/remko/go-mkvparse"
)

// decodeVINT decodes an EBML variable-length integer from b.
// Returns the value, the number of bytes consumed, and any error.
func decodeVINT(b []byte) (value uint64, length int, err error) {
	if len(b) == 0 {
		return 0, 0, errors.New("empty buffer")
	}
	first := b[0]
	if first == 0 {
		return 0, 0, errors.New("invalid vint: first byte is zero")
	}
	// Count leading zeros to determine width
	length = 1
	mask := byte(0x80)
	for mask != 0 && first&mask == 0 {
		length++
		mask >>= 1
	}
	if length > 8 || length > len(b) {
		return 0, 0, fmt.Errorf("vint too long: length=%d buflen=%d", length, len(b))
	}
	// Strip the marker bit from the first byte
	value = uint64(first &^ mask)
	for i := 1; i < length; i++ {
		value = (value << 8) | uint64(b[i])
	}
	return value, length, nil
}

// assEvent holds a single parsed subtitle event before final formatting.
type assEvent struct {
	startNs int64
	endNs   int64
	payload string // raw MKV payload (ReadOrder,Layer,Style,...)
}

// assHandler implements mkvparse.Handler to extract a single ASS/SSA track.
type assHandler struct {
	mkvparse.DefaultHandler

	// Configuration
	targetSubIndex int // which subtitle-relative track to extract (0-based)

	// State: global
	timestampScale uint64 // nanoseconds per tick, default 1_000_000

	// State: track enumeration
	currentSubIndex int    // how many subtitle tracks we've passed
	inTrackEntry    bool   // are we inside a TrackEntry element?
	curTrackNumber  uint64
	curTrackType    int64
	curCodecID      string
	curCodecPrivate []byte

	// State: target track
	targetTrackNumber uint64
	targetCodecPrivate []byte
	targetFound        bool

	// State: cluster
	clusterTimestamp int64

	// State: block group
	inBlockGroup     bool
	pendingBlock     []byte // raw block bytes (track+timestamp+flags+payload)
	pendingDuration  uint64 // BlockDuration in ticks

	// Collected events
	events []assEvent
}

func (h *assHandler) HandleMasterBegin(id mkvparse.ElementID, info mkvparse.ElementInfo) (bool, error) {
	switch id {
	case mkvparse.TrackEntryElement:
		h.inTrackEntry = true
		h.curTrackNumber = 0
		h.curTrackType = 0
		h.curCodecID = ""
		h.curCodecPrivate = nil
	case mkvparse.BlockGroupElement:
		h.inBlockGroup = true
		h.pendingBlock = nil
		h.pendingDuration = 0
	}
	return true, nil
}

func (h *assHandler) HandleMasterEnd(id mkvparse.ElementID, info mkvparse.ElementInfo) error {
	switch id {
	case mkvparse.TrackEntryElement:
		if h.inTrackEntry && h.curTrackType == 17 {
			codecID := h.curCodecID
			if strings.HasPrefix(codecID, "S_TEXT/ASS") || strings.HasPrefix(codecID, "S_TEXT/SSA") {
				if h.currentSubIndex == h.targetSubIndex {
					h.targetTrackNumber = h.curTrackNumber
					h.targetCodecPrivate = h.curCodecPrivate
					h.targetFound = true
				}
				h.currentSubIndex++
			}
		}
		h.inTrackEntry = false
	case mkvparse.BlockGroupElement:
		if h.inBlockGroup && h.pendingBlock != nil && h.targetFound {
			h.saveBlockEvent(h.pendingBlock, h.pendingDuration)
		}
		h.inBlockGroup = false
		h.pendingBlock = nil
		h.pendingDuration = 0
	}
	return nil
}

func (h *assHandler) HandleString(id mkvparse.ElementID, value string, info mkvparse.ElementInfo) error {
	if id == mkvparse.CodecIDElement && h.inTrackEntry {
		h.curCodecID = value
	}
	return nil
}

func (h *assHandler) HandleInteger(id mkvparse.ElementID, value int64, info mkvparse.ElementInfo) error {
	switch id {
	case mkvparse.TimecodeScaleElement:
		h.timestampScale = uint64(value)
	case mkvparse.TrackNumberElement:
		if h.inTrackEntry {
			h.curTrackNumber = uint64(value)
		}
	case mkvparse.TrackTypeElement:
		if h.inTrackEntry {
			h.curTrackType = value
		}
	case mkvparse.TimecodeElement:
		// Cluster timestamp (base time for all blocks in this cluster)
		h.clusterTimestamp = value
	case mkvparse.BlockDurationElement:
		if h.inBlockGroup {
			h.pendingDuration = uint64(value)
		}
	}
	return nil
}

func (h *assHandler) HandleBinary(id mkvparse.ElementID, value []byte, info mkvparse.ElementInfo) error {
	switch id {
	case mkvparse.CodecPrivateElement:
		if h.inTrackEntry {
			cp := make([]byte, len(value))
			copy(cp, value)
			h.curCodecPrivate = cp
		}
	case mkvparse.BlockElement:
		if h.targetFound && h.inBlockGroup {
			blk := make([]byte, len(value))
			copy(blk, value)
			h.pendingBlock = blk
		}
	case mkvparse.SimpleBlockElement:
		if h.targetFound {
			// SimpleBlock: decode and save immediately with default duration
			h.saveBlockEvent(value, 0)
		}
	}
	return nil
}

// saveBlockEvent decodes a raw SimpleBlock/Block binary, checks if it belongs
// to our target track, and appends an event. durationTicks=0 means unknown.
func (h *assHandler) saveBlockEvent(data []byte, durationTicks uint64) {
	trackNum, vlen, err := decodeVINT(data)
	if err != nil || vlen >= len(data) {
		return
	}
	if trackNum != h.targetTrackNumber {
		return
	}
	data = data[vlen:]
	if len(data) < 3 {
		return
	}
	// 2 bytes big-endian int16 relative timestamp
	relTimestamp := int16(binary.BigEndian.Uint16(data[:2]))
	// 1 byte flags (skip)
	payload := string(data[3:])

	// Compute absolute times in nanoseconds
	scale := int64(h.timestampScale)
	if scale == 0 {
		scale = 1_000_000
	}
	startNs := (h.clusterTimestamp+int64(relTimestamp))*scale
	var endNs int64
	if durationTicks > 0 {
		endNs = startNs + int64(durationTicks)*scale
	} else {
		// Fallback: 5-second duration for SimpleBlock without duration info
		endNs = startNs + 5_000_000_000
	}

	h.events = append(h.events, assEvent{
		startNs: startNs,
		endNs:   endNs,
		payload: payload,
	})
}

// formatTime formats nanoseconds as H:MM:SS.cc (ASS time format).
func formatTime(ns int64) string {
	if ns < 0 {
		ns = 0
	}
	totalCs := ns / 10_000_000 // centiseconds
	cs := totalCs % 100
	totalSec := totalCs / 100
	secs := totalSec % 60
	totalMin := totalSec / 60
	mins := totalMin % 60
	hours := totalMin / 60
	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, mins, secs, cs)
}

// ExtractASSTrack extracts an ASS/SSA subtitle track from an MKV file and
// writes a valid .ass file to outputPath. trackIndex is the subtitle-relative
// index (0 = first subtitle track, 1 = second, etc.), matching the same
// indexing we use for ffmpeg's -map 0:s:N.
// Returns an error if the track is not found or is not ASS/SSA.
func ExtractASSTrack(videoPath string, trackIndex int, outputPath string) error {
	handler := &assHandler{
		targetSubIndex: trackIndex,
		timestampScale: 1_000_000, // default: 1 tick = 1 ms = 1,000,000 ns
	}

	if err := mkvparse.ParsePath(videoPath, handler); err != nil {
		return fmt.Errorf("mkv parse error: %w", err)
	}

	if !handler.targetFound {
		return fmt.Errorf("ASS/SSA subtitle track %d not found in %s", trackIndex, videoPath)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	// Write CodecPrivate (ASS header without [Events])
	header := handler.targetCodecPrivate
	if _, err := f.Write(header); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Ensure trailing newline before appending [Events]
	if len(header) > 0 && header[len(header)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("writing newline: %w", err)
		}
	}

	// Write [Events] section header
	eventsHeader := "\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"
	if _, err := f.WriteString(eventsHeader); err != nil {
		return fmt.Errorf("writing events header: %w", err)
	}

	// Write dialogue lines
	for _, ev := range handler.events {
		line, err := formatDialogue(ev)
		if err != nil {
			continue // skip malformed events
		}
		if _, err := f.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("writing dialogue: %w", err)
		}
	}

	return nil
}

// formatDialogue converts a raw MKV ASS event payload into a Dialogue line.
// MKV payload format: ReadOrder,Layer,Style,Name,MarginL,MarginR,MarginV,Effect,Text
// Output: Dialogue: Layer,Start,End,Style,Name,MarginL,MarginR,MarginV,Effect,Text
func formatDialogue(ev assEvent) (string, error) {
	// Split on commas, but only the first 8 (Text field may contain commas)
	const numFields = 9 // ReadOrder + 8 more fields
	parts := strings.SplitN(ev.payload, ",", numFields)
	if len(parts) < numFields {
		return "", fmt.Errorf("too few fields in ASS payload: %q", ev.payload)
	}
	// parts[0] = ReadOrder (discard)
	// parts[1] = Layer
	// parts[2] = Style
	// parts[3] = Name
	// parts[4] = MarginL
	// parts[5] = MarginR
	// parts[6] = MarginV
	// parts[7] = Effect
	// parts[8] = Text (may contain commas, that's fine — SplitN keeps it whole)
	layer := parts[1]
	style := parts[2]
	name := parts[3]
	marginL := parts[4]
	marginR := parts[5]
	marginV := parts[6]
	effect := parts[7]
	text := parts[8]

	return fmt.Sprintf("Dialogue: %s,%s,%s,%s,%s,%s,%s,%s,%s,%s",
		layer,
		formatTime(ev.startNs),
		formatTime(ev.endNs),
		style,
		name,
		marginL,
		marginR,
		marginV,
		effect,
		text,
	), nil
}

// Ensure assHandler satisfies the Handler interface at compile time.
var _ mkvparse.Handler = (*assHandler)(nil)

// Ensure time import is used (needed for DefaultHandler embedding).
var _ = time.Time{}
