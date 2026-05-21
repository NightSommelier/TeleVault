package telegramartifact

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
)

const (
	wrapperMagic                = "TVW1"
	wrapperVersionPlain         = 0x01
	wrapperVersionXOR           = 0x02
	ageMagicPrefix              = "age-encryption.org/v1"
	smallArtifactMaxSize  int64 = 8 * 1024 * 1024
	mediumArtifactMaxSize int64 = 128 * 1024 * 1024
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

func makeProfile(name, extension, mimeType string, prefix []byte) DecoyProfile {
	return DecoyProfile{Name: name, Extension: extension, MIMEType: mimeType, Prefix: prefix}
}

var smallProfiles = []DecoyProfile{
	makeProfile("image-jpeg", ".jpg", "image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00}),
	makeProfile("image-jpeg-alt", ".jpeg", "image/jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00}),
	makeProfile("image-png", ".png", "image/png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}),
	makeProfile("image-gif", ".gif", "image/gif", []byte("GIF89a")),
	makeProfile("image-webp", ".webp", "image/webp", []byte{'R', 'I', 'F', 'F', 0x00, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P'}),
	makeProfile("document-pdf", ".pdf", "application/pdf", []byte("%PDF-1.7\n%TeleVault\n")),
}

var mediumProfiles = []DecoyProfile{
	makeProfile("audio-mpeg", ".mp3", "audio/mpeg", []byte("ID3\x04\x00\x00\x00\x00\x00\x00")),
	makeProfile("audio-m4a", ".m4a", "audio/mp4", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'M', '4', 'A', ' ', 0x00, 0x00, 0x00, 0x00, 'M', '4', 'A', ' ', 'i', 's', 'o', 'm'}),
	makeProfile("document-docx", ".docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")),
	makeProfile("document-xlsx", ".xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")),
	makeProfile("document-pptx", ".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")),
	makeProfile("archive-zip", ".zip", "application/zip", []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00")),
	makeProfile("archive-rar", ".rar", "application/vnd.rar", []byte("Rar!\x1A\x07\x00")),
}

var largeProfiles = []DecoyProfile{
	makeProfile("video-mp4", ".mp4", "video/mp4", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0x00, 0x00, 0x00, 0x00, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}),
	makeProfile("video-m4v", ".m4v", "video/x-m4v", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'M', '4', 'V', ' ', 0x00, 0x00, 0x00, 0x00, 'M', '4', 'V', ' ', 'i', 's', 'o', 'm'}),
	makeProfile("video-mkv", ".mkv", "video/x-matroska", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x93, 0x42, 0x86, 'm', 'a', 't', 'r', 'o', 's', 'k', 'a'}),
	makeProfile("video-avi", ".avi", "video/x-msvideo", []byte("RIFF\x00\x00\x00\x00AVI ")),
	makeProfile("video-3gp", ".3gp", "video/3gpp", []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', '3', 'g', 'p', '5', 0x00, 0x00, 0x00, 0x00, '3', 'g', 'p', '5', 'i', 's', 'o', 'm'}),
	makeProfile("archive-7z", ".7z", "application/x-7z-compressed", []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}),
	makeProfile("archive-bin", ".bin", "application/octet-stream", []byte("TVBIN\x01\x00")),
}

func SpecForArtifactID(artifactID string) ArtifactSpec {
	return SpecForArtifactIDAndSize(artifactID, 0)
}

func SpecForArtifactIDAndSize(artifactID string, size int64) ArtifactSpec {
	candidates := profilesForSize(size)
	if len(candidates) == 0 {
		return ArtifactSpec{ArtifactID: artifactID}
	}
	idx := stableIndex(artifactID, len(candidates))
	return ArtifactSpec{
		ArtifactID:   artifactID,
		ProfileIndex: idx,
		Profile:      candidates[idx],
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
	return WrapReaderForSize(artifactID, 0, src)
}

func WrapReaderForSize(artifactID string, size int64, src io.Reader) io.Reader {
	spec := SpecForArtifactIDAndSize(artifactID, size)
	return io.MultiReader(
		bytes.NewReader(spec.Profile.Prefix),
		bytes.NewReader(wrapperHeader(spec.ProfileIndex, wrapperVersionXOR)),
		newMaskReader(src, spec.ProfileIndex),
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

	spec, version, ok := matchWrappedSpec(peek)
	if !ok {
		return nil, errors.New("telegram artifact wrapper not recognized")
	}

	consume := len(spec.Profile.Prefix) + len(wrapperHeader(spec.ProfileIndex, version))
	if _, err := br.Discard(consume); err != nil {
		return nil, err
	}
	if version == wrapperVersionXOR {
		return newMaskReader(br, spec.ProfileIndex), nil
	}
	return br, nil
}

func matchWrappedSpec(peek []byte) (ArtifactSpec, byte, bool) {
	for _, spec := range wrappedSpecs() {
		for _, version := range []byte{wrapperVersionXOR, wrapperVersionPlain} {
			header := wrapperHeader(spec.ProfileIndex, version)
			expected := append([]byte{}, spec.Profile.Prefix...)
			expected = append(expected, header...)
			if bytes.HasPrefix(peek, expected) {
				return spec, version, true
			}
		}
	}
	return ArtifactSpec{}, 0, false
}

func wrappedSpecs() []ArtifactSpec {
	groups := [][]DecoyProfile{smallProfiles, mediumProfiles, largeProfiles}
	count := len(allProfiles()) + len(smallProfiles) + len(mediumProfiles) + len(largeProfiles)
	specs := make([]ArtifactSpec, 0, count)
	for _, profiles := range groups {
		for idx, profile := range profiles {
			specs = append(specs, ArtifactSpec{ProfileIndex: idx, Profile: profile})
		}
	}
	for idx, profile := range allProfiles() {
		specs = append(specs, ArtifactSpec{ProfileIndex: idx, Profile: profile})
	}
	return specs
}

func wrapperHeader(profileIndex int, version byte) []byte {
	return []byte{wrapperMagic[0], wrapperMagic[1], wrapperMagic[2], wrapperMagic[3], byte(profileIndex), version}
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
	for _, profile := range allProfiles() {
		length := len(profile.Prefix) + len(wrapperHeader(0, wrapperVersionXOR))
		if length > maxLen {
			maxLen = length
		}
	}
	return maxLen
}

func profilesForSize(size int64) []DecoyProfile {
	switch {
	case size <= 0, size <= smallArtifactMaxSize:
		return smallProfiles
	case size <= mediumArtifactMaxSize:
		return mediumProfiles
	default:
		return largeProfiles
	}
}

func allProfiles() []DecoyProfile {
	profiles := make([]DecoyProfile, 0, len(smallProfiles)+len(mediumProfiles)+len(largeProfiles))
	profiles = append(profiles, smallProfiles...)
	profiles = append(profiles, mediumProfiles...)
	profiles = append(profiles, largeProfiles...)
	return profiles
}

type maskReader struct {
	src     io.Reader
	key     [32]byte
	offset  uint64
	block   [32]byte
	blockID uint64
}

func newMaskReader(src io.Reader, profileIndex int) io.Reader {
	key := sha256.Sum256([]byte(fmt.Sprintf("televault-artifact-mask:v1:%d", profileIndex)))
	return &maskReader{src: src, key: key}
}

func (r *maskReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	for i := 0; i < n; i++ {
		p[i] ^= r.nextMaskByte()
	}
	return n, err
}

func (r *maskReader) nextMaskByte() byte {
	blockID := r.offset / uint64(len(r.block))
	if r.offset%uint64(len(r.block)) == 0 || blockID != r.blockID {
		seed := make([]byte, 40)
		copy(seed, r.key[:])
		for i := 0; i < 8; i++ {
			seed[32+i] = byte(blockID >> (8 * i))
		}
		r.block = sha256.Sum256(seed)
		r.blockID = blockID
	}
	value := r.block[r.offset%uint64(len(r.block))]
	r.offset++
	return value
}

func (s ArtifactSpec) String() string {
	return fmt.Sprintf("%s:%s", s.Profile.Name, s.ArtifactID)
}
