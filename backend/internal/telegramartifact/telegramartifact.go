package telegramartifact

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
)

const (
	wrapperMagic   = "TVW1"
	ageMagicPrefix = "age-encryption.org/v1"
)

type DecoyProfile struct {
	Name      string
	Extension string
	MIMEType  string
	Prefix    []byte
}

type ArtifactSpec struct {
	ArtifactID   string
	ProfileIndex int
	Profile      DecoyProfile
}

var profiles = []DecoyProfile{
	{
		Name:      "audio-mpeg",
		Extension: ".mp3",
		MIMEType:  "audio/mpeg",
		Prefix:    []byte("ID3\x04\x00\x00\x00\x00\x00\x00"),
	},
	{
		Name:      "video-mp4",
		Extension: ".mp4",
		MIMEType:  "video/mp4",
		Prefix:    []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'},
	},
	{
		Name:      "video-m4v",
		Extension: ".m4v",
		MIMEType:  "video/x-m4v",
		Prefix:    []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'M', '4', 'V', ' ', 0x00, 0x00, 0x00, 0x00, 'M', '4', 'V', ' ', 'i', 's', 'o', 'm'},
	},
	{
		Name:      "video-avi",
		Extension: ".avi",
		MIMEType:  "video/x-msvideo",
		Prefix:    []byte("RIFF\x00\x00\x00\x00AVI "),
	},
	{
		Name:      "video-3gp",
		Extension: ".3gp",
		MIMEType:  "video/3gpp",
		Prefix:    []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', '3', 'g', 'p', '5', 0x00, 0x00, 0x00, 0x00, '3', 'g', 'p', '5', 'i', 's', 'o', 'm'},
	},
	{
		Name:      "image-jpeg",
		Extension: ".jpg",
		MIMEType:  "image/jpeg",
		Prefix:    []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00},
	},
	{
		Name:      "image-jpeg-alt",
		Extension: ".jpeg",
		MIMEType:  "image/jpeg",
		Prefix:    []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00},
	},
	{
		Name:      "archive-zip",
		Extension: ".zip",
		MIMEType:  "application/zip",
		Prefix:    []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00"),
	},
	{
		Name:      "document-pdf",
		Extension: ".pdf",
		MIMEType:  "application/pdf",
		Prefix:    []byte("%PDF-1.7\n%TeleVault\n"),
	},
}

func SpecForArtifactID(artifactID string) ArtifactSpec {
	if len(profiles) == 0 {
		return ArtifactSpec{ArtifactID: artifactID}
	}
	idx := stableIndex(artifactID, len(profiles))
	return ArtifactSpec{
		ArtifactID:   artifactID,
		ProfileIndex: idx,
		Profile:      profiles[idx],
	}
}

func (s ArtifactSpec) Name() string {
	if s.ArtifactID == "" {
		return "artifact" + s.Profile.Extension
	}
	return s.ArtifactID + s.Profile.Extension
}

func (s ArtifactSpec) MIMEType() string {
	if s.Profile.MIMEType == "" {
		return "application/octet-stream"
	}
	return s.Profile.MIMEType
}

func WrapReader(artifactID string, src io.Reader) io.Reader {
	spec := SpecForArtifactID(artifactID)
	return io.MultiReader(
		bytes.NewReader(spec.Profile.Prefix),
		bytes.NewReader(wrapperHeader(spec.ProfileIndex)),
		src,
	)
}

func UnwrapReader(src io.Reader) (io.Reader, error) {
	br := bufio.NewReader(src)
	peekLen := maxDetectBytes()
	peek, err := br.Peek(peekLen)
	if err != nil && !errors.Is(err, bufio.ErrBufferFull) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if bytes.HasPrefix(peek, []byte(ageMagicPrefix)) {
		return br, nil
	}

	spec, ok := matchWrappedSpec(peek)
	if !ok {
		return nil, errors.New("telegram artifact wrapper not recognized")
	}

	consume := len(spec.Profile.Prefix) + len(wrapperHeader(spec.ProfileIndex))
	if _, err := br.Discard(consume); err != nil {
		return nil, err
	}
	return br, nil
}

func matchWrappedSpec(peek []byte) (ArtifactSpec, bool) {
	for idx, profile := range profiles {
		spec := ArtifactSpec{ProfileIndex: idx, Profile: profile}
		header := wrapperHeader(idx)
		expected := append([]byte{}, profile.Prefix...)
		expected = append(expected, header...)
		if bytes.HasPrefix(peek, expected) {
			return spec, true
		}
	}
	return ArtifactSpec{}, false
}

func wrapperHeader(profileIndex int) []byte {
	return []byte{wrapperMagic[0], wrapperMagic[1], wrapperMagic[2], wrapperMagic[3], byte(profileIndex), 0x01}
}

func stableIndex(key string, modulo int) int {
	if modulo <= 0 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum64() % uint64(modulo))
}

func maxDetectBytes() int {
	maxLen := len(ageMagicPrefix)
	for _, profile := range profiles {
		length := len(profile.Prefix) + len(wrapperHeader(0))
		if length > maxLen {
			maxLen = length
		}
	}
	return maxLen
}

func (s ArtifactSpec) String() string {
	return fmt.Sprintf("%s:%s", s.Profile.Name, s.ArtifactID)
}
