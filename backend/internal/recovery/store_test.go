package recovery

import (
	"errors"
	"regexp"
	"testing"
)

func TestNewUUIDFormat(t *testing.T) {
	id, err := newUUID()
	if err != nil {
		t.Fatalf("newUUID() error = %v", err)
	}
	matched, err := regexp.MatchString(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, id)
	if err != nil {
		t.Fatalf("regexp error = %v", err)
	}
	if !matched {
		t.Fatalf("newUUID() = %q, want RFC 4122 version 4 shape", id)
	}
}

func TestValidateManifestRejectsUnsafeShapes(t *testing.T) {
	valid := testManifest()
	cases := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{
			name: "duplicate file id",
			mutate: func(manifest *Manifest) {
				manifest.Files = append(manifest.Files, manifest.Files[0])
			},
		},
		{
			name: "owner mismatch",
			mutate: func(manifest *Manifest) {
				manifest.Files[0].OwnerID = "other-user"
			},
		},
		{
			name: "folder parts",
			mutate: func(manifest *Manifest) {
				manifest.Files[0].Parts = []PartEntry{testPart("part-folder", 1)}
			},
		},
		{
			name: "duplicate part number",
			mutate: func(manifest *Manifest) {
				manifest.Files[1].Parts = append(manifest.Files[1].Parts, testPart("part-2", 1))
			},
		},
		{
			name: "bad part range",
			mutate: func(manifest *Manifest) {
				end := int64(5)
				manifest.Files[1].Parts[0].PlaintextEnd = &end
			},
		},
		{
			name: "parent cycle",
			mutate: func(manifest *Manifest) {
				manifest.Files[0].ParentID = manifest.Files[1].ID
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manifest := valid
			manifest.Files = cloneFiles(valid.Files)
			tc.mutate(&manifest)
			if err := validateManifest(manifest); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("validateManifest() error = %v, want ErrInvalidManifest", err)
			}
		})
	}
}

func TestValidateManifestAcceptsRecoveryShape(t *testing.T) {
	if err := validateManifest(testManifest()); err != nil {
		t.Fatalf("validateManifest() error = %v", err)
	}
}

func testManifest() Manifest {
	return Manifest{
		Schema:          ManifestSchema,
		SnapshotID:      "snapshot-1",
		SnapshotVersion: 1,
		User: UserEntry{
			ID:         "user-1",
			TelegramID: 1001,
		},
		Files: []FileEntry{
			{
				ID:      "folder-1",
				OwnerID: "user-1",
				Type:    "folder",
				Status:  "ready",
			},
			{
				ID:       "file-1",
				OwnerID:  "user-1",
				ParentID: "folder-1",
				Type:     "file",
				Status:   "ready",
				Parts:    []PartEntry{testPart("part-1", 1)},
			},
		},
	}
}

func testPart(id string, number int) PartEntry {
	start := int64(0)
	end := int64(10)
	size := int64(10)
	return PartEntry{
		ID:                id,
		PartNumber:        number,
		PlaintextStart:    &start,
		PlaintextEnd:      &end,
		PlaintextSize:     &size,
		TelegramPeer:      "self",
		TelegramMessageID: 101,
		CiphertextSize:    42,
	}
}

func cloneFiles(files []FileEntry) []FileEntry {
	out := make([]FileEntry, len(files))
	copy(out, files)
	for i := range out {
		if len(files[i].Parts) == 0 {
			continue
		}
		out[i].Parts = append([]PartEntry(nil), files[i].Parts...)
	}
	return out
}
