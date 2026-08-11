package decodecheck

import "testing"

func TestIsTimestampError(t *testing.T) {
	cases := []string{
		"non monotonically increasing dts to muxer",
		"timestamp discontinuity detected",
		"negative timestamp at packet",
		"Non-monotonous DTS in output stream",
	}
	for _, item := range cases {
		if !isTimestampError(item) {
			t.Fatalf("expected timestamp error for %q", item)
		}
	}
}

func TestIsTimestampErrorIgnoresDecodeNoise(t *testing.T) {
	if isTimestampError("error while decoding MB 12 34") {
		t.Fatalf("decode noise should not be classified as timestamp error")
	}
}
