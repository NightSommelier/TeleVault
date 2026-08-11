package recovery

import (
	"errors"
	"regexp"
	"testing"

	"filippo.io/age"
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
		{
			name: "unsupported storage backend",
			mutate: func(manifest *Manifest) {
				manifest.Files[1].Parts[0].StorageBackend = "s3"
			},
		},
		{
			name: "telegram storage locator mismatch",
			mutate: func(manifest *Manifest) {
				part := &manifest.Files[1].Parts[0]
				part.StorageLocator = "other-peer"
			},
		},
		{
			name: "telegram storage owner mismatch",
			mutate: func(manifest *Manifest) {
				part := &manifest.Files[1].Parts[0]
				part.StorageOwnerUser = "other-user"
			},
		},
		{
			name: "legacy storage locator mismatch",
			mutate: func(manifest *Manifest) {
				part := &manifest.Files[1].Parts[0]
				part.StorageBackend = ""
				part.StorageLocator = "other-peer"
			},
		},
		{
			name: "legacy storage owner mismatch",
			mutate: func(manifest *Manifest) {
				part := &manifest.Files[1].Parts[0]
				part.StorageBackend = ""
				part.StorageOwnerUser = "other-user"
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

func TestValidateManifestAcceptsLegacyStorageShape(t *testing.T) {
	manifest := testManifest()
	part := &manifest.Files[1].Parts[0]
	part.StorageBackend = ""
	part.StorageLocator = ""
	part.StorageOwnerUser = ""
	if err := validateManifest(manifest); err != nil {
		t.Fatalf("validateManifest() error = %v", err)
	}
}

func testManifest() Manifest {
	return Manifest{
		Schema:          ManifestSchema,
		InstanceID:      "instance-1",
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
		StorageBackend:    "telegram",
		StorageLocator:    "self",
		StorageOwnerUser:  "user-1",
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

func TestResolveImportIdentity(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("age.GenerateX25519Identity() error = %v", err)
	}
	publicRecipient := identity.Recipient().String()

	t.Run("private key provided", func(t *testing.T) {
		resolved, shouldImport, err := resolveImportIdentity(UserEntry{
			AgePublicRecipient: publicRecipient,
			AgePrivateIdentity: identity.String(),
		}, nil, ImportModeMerge)
		if err != nil {
			t.Fatalf("resolveImportIdentity() error = %v", err)
		}
		if !shouldImport {
			t.Fatalf("resolveImportIdentity() shouldImport = false, want true")
		}
		if resolved == nil || resolved.Recipient().String() != publicRecipient {
			t.Fatalf("resolveImportIdentity() resolved = %#v", resolved)
		}
	})

	t.Run("missing private key but matching existing key", func(t *testing.T) {
		resolved, shouldImport, err := resolveImportIdentity(UserEntry{
			AgePublicRecipient: publicRecipient,
		}, &userKey{PublicRecipient: publicRecipient}, ImportModeMerge)
		if err != nil {
			t.Fatalf("resolveImportIdentity() error = %v", err)
		}
		if shouldImport {
			t.Fatalf("resolveImportIdentity() shouldImport = true, want false")
		}
		if resolved != nil {
			t.Fatalf("resolveImportIdentity() resolved = %#v, want nil", resolved)
		}
	})

	t.Run("missing private key without existing key", func(t *testing.T) {
		_, _, err := resolveImportIdentity(UserEntry{
			AgePublicRecipient: publicRecipient,
		}, nil, ImportModeMerge)
		if !errors.Is(err, ErrInvalidManifest) {
			t.Fatalf("resolveImportIdentity() error = %v, want ErrInvalidManifest", err)
		}
	})

	t.Run("missing private key with different existing key", func(t *testing.T) {
		other, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatalf("age.GenerateX25519Identity() error = %v", err)
		}
		_, _, err = resolveImportIdentity(UserEntry{
			AgePublicRecipient: publicRecipient,
		}, &userKey{PublicRecipient: other.Recipient().String()}, ImportModeMerge)
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("resolveImportIdentity() error = %v, want ErrConflict", err)
		}
	})

	t.Run("missing private key with different existing key in replace mode", func(t *testing.T) {
		other, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatalf("age.GenerateX25519Identity() error = %v", err)
		}
		resolved, shouldImport, err := resolveImportIdentity(UserEntry{
			AgePublicRecipient: publicRecipient,
		}, &userKey{PublicRecipient: other.Recipient().String()}, ImportModeReplace)
		if err != nil {
			t.Fatalf("resolveImportIdentity() error = %v", err)
		}
		if shouldImport {
			t.Fatalf("resolveImportIdentity() shouldImport = true, want false")
		}
		if resolved != nil {
			t.Fatalf("resolveImportIdentity() resolved = %#v, want nil", resolved)
		}
	})

	t.Run("private key with different existing key in replace mode", func(t *testing.T) {
		other, err := age.GenerateX25519Identity()
		if err != nil {
			t.Fatalf("age.GenerateX25519Identity() error = %v", err)
		}
		resolved, shouldImport, err := resolveImportIdentity(UserEntry{
			AgePublicRecipient: publicRecipient,
			AgePrivateIdentity: identity.String(),
		}, &userKey{PublicRecipient: other.Recipient().String()}, ImportModeReplace)
		if err != nil {
			t.Fatalf("resolveImportIdentity() error = %v", err)
		}
		if shouldImport {
			t.Fatalf("resolveImportIdentity() shouldImport = true, want false")
		}
		if resolved != nil {
			t.Fatalf("resolveImportIdentity() resolved = %#v, want nil", resolved)
		}
	})
}

func TestNormalizeImportOptions(t *testing.T) {
	tests := []struct {
		name    string
		input   ImportOptions
		want    string
		confirm bool
	}{
		{name: "default merge", input: ImportOptions{}, want: ImportModeMerge},
		{name: "replace", input: ImportOptions{Mode: ImportModeReplace, ConfirmReplace: true}, want: ImportModeReplace, confirm: true},
		{name: "invalid mode fallback", input: ImportOptions{Mode: "unexpected"}, want: ImportModeMerge},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeImportOptions(tc.input)
			if got.Mode != tc.want {
				t.Fatalf("normalizeImportOptions().Mode = %q, want %q", got.Mode, tc.want)
			}
			if got.ConfirmReplace != tc.confirm {
				t.Fatalf("normalizeImportOptions().ConfirmReplace = %v, want %v", got.ConfirmReplace, tc.confirm)
			}
		})
	}
}
