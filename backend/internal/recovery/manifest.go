package recovery

import "time"

const ManifestSchema = "televault.recovery.v1"

type Manifest struct {
	Schema          string      `json:"schema"`
	InstanceID      string      `json:"instance_id,omitempty"`
	SnapshotID      string      `json:"snapshot_id"`
	SnapshotVersion int         `json:"snapshot_version"`
	ExportedAt      time.Time   `json:"exported_at"`
	User            UserEntry   `json:"user"`
	Files           []FileEntry `json:"files"`
}

type UserEntry struct {
	ID                 string `json:"id"`
	TelegramID         int64  `json:"telegram_id"`
	Username           string `json:"username,omitempty"`
	DisplayName        string `json:"display_name,omitempty"`
	AgePublicRecipient string `json:"age_public_recipient"`
	AgePrivateIdentity string `json:"age_private_identity"`
}

type FileEntry struct {
	ID             string      `json:"id"`
	OwnerID        string      `json:"owner_id"`
	ParentID       string      `json:"parent_id,omitempty"`
	NamePlain      string      `json:"name_plain,omitempty"`
	MimeType       string      `json:"mime_type,omitempty"`
	PlaintextSize  *int64      `json:"plaintext_size,omitempty"`
	CiphertextSize *int64      `json:"ciphertext_size,omitempty"`
	Type           string      `json:"type"`
	Status         string      `json:"status"`
	Checksum       []byte      `json:"checksum,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	DeletedAt      *time.Time  `json:"deleted_at,omitempty"`
	Parts          []PartEntry `json:"parts,omitempty"`
}

type PartEntry struct {
	ID                string    `json:"id"`
	PartNumber        int       `json:"part_number"`
	PlaintextStart    *int64    `json:"plaintext_start,omitempty"`
	PlaintextEnd      *int64    `json:"plaintext_end,omitempty"`
	PlaintextSize     *int64    `json:"plaintext_size,omitempty"`
	StorageBackend    string    `json:"storage_backend,omitempty"`
	StorageLocator    string    `json:"storage_locator,omitempty"`
	StorageOwnerUser  string    `json:"storage_owner_user_id,omitempty"`
	TelegramPeer      string    `json:"telegram_peer"`
	TelegramMessageID int64     `json:"telegram_message_id"`
	CiphertextSize    int64     `json:"ciphertext_size"`
	Checksum          []byte    `json:"checksum,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}
