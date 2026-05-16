package uploads

import "testing"

func TestNormalizeName(t *testing.T) {
	tests := map[string]string{
		" Report.pdf ": "Report.pdf",
		"/bad/name/":   "badname",
		"   ":          "",
	}

	for input, want := range tests {
		if got := normalizeName(input); got != want {
			t.Fatalf("normalizeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	algorithm, checksum, err := parseChecksum("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("parseChecksum returned error: %v", err)
	}
	if algorithm != "sha256" {
		t.Fatalf("algorithm = %q, want sha256", algorithm)
	}
	if len(checksum) != 32 {
		t.Fatalf("checksum length = %d, want 32", len(checksum))
	}
}

func TestParseChecksumRejectsInvalidValue(t *testing.T) {
	if _, _, err := parseChecksum("abc"); err == nil {
		t.Fatal("parseChecksum accepted invalid checksum")
	}
}

func TestPartCount(t *testing.T) {
	partSize := int64(64 * 1024 * 1024)
	tests := map[int64]int64{
		0:            0,
		1:            1,
		partSize:     1,
		partSize + 1: 2,
	}

	for size, want := range tests {
		if got := partCount(size, partSize); got != want {
			t.Fatalf("partCount(%d) = %d, want %d", size, got, want)
		}
	}
}

func TestUploadPartResponseIncludesChecksumHex(t *testing.T) {
	part := UploadPart{
		Checksum: []byte{0xde, 0xad, 0xbe, 0xef},
	}

	response := uploadPartResponse(part)
	if response["checksum"] != "deadbeef" {
		t.Fatalf("checksum = %v, want deadbeef", response["checksum"])
	}
}
