package mkv

import "testing"

func TestDecodeVINT(t *testing.T) {
	tests := []struct {
		input   []byte
		want    uint64
		wantLen int
	}{
		{[]byte{0x81}, 1, 1},
		{[]byte{0x82}, 2, 1},
		{[]byte{0xFF}, 127, 1},
		{[]byte{0x40, 0x80}, 128, 2},
		{[]byte{0x41, 0x00}, 256, 2},
	}
	for _, tt := range tests {
		got, gotLen, err := decodeVINT(tt.input)
		if err != nil {
			t.Errorf("decodeVINT(%x) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("decodeVINT(%x) = %d, want %d", tt.input, got, tt.want)
		}
		if gotLen != tt.wantLen {
			t.Errorf("decodeVINT(%x) length = %d, want %d", tt.input, gotLen, tt.wantLen)
		}
	}
}

func TestDecodeVINTErrors(t *testing.T) {
	// Empty buffer
	_, _, err := decodeVINT([]byte{})
	if err == nil {
		t.Error("expected error for empty buffer")
	}

	// Zero first byte
	_, _, err = decodeVINT([]byte{0x00})
	if err == nil {
		t.Error("expected error for zero first byte")
	}
}

func TestFormatTime(t *testing.T) {
	tests := []struct {
		ns   int64
		want string
	}{
		{0, "0:00:00.00"},
		{1_000_000_000, "0:00:01.00"},  // 1 second
		{60_000_000_000, "0:01:00.00"}, // 1 minute
		{3_600_000_000_000, "1:00:00.00"}, // 1 hour
		{10_000_000, "0:00:00.01"},        // 1 centisecond
		{1_234_560_000, "0:00:01.23"},     // 1.23 seconds
		{-100, "0:00:00.00"},              // negative clamps to 0
	}
	for _, tt := range tests {
		got := formatTime(tt.ns)
		if got != tt.want {
			t.Errorf("formatTime(%d) = %q, want %q", tt.ns, got, tt.want)
		}
	}
}
