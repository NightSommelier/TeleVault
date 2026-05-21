package files

import "sync"

type DownloadActivity struct {
	Total  int `json:"total"`
	Auth   int `json:"auth"`
	Public int `json:"public"`
}

type DownloadTracker struct {
	mu     sync.RWMutex
	byFile map[string]DownloadActivity
}

func NewDownloadTracker() *DownloadTracker {
	return &DownloadTracker{
		byFile: make(map[string]DownloadActivity),
	}
}

func (t *DownloadTracker) Increment(fileID string, source string) {
	if t == nil || fileID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	activity := t.byFile[fileID]
	activity.Total++
	switch source {
	case "public":
		activity.Public++
	default:
		activity.Auth++
	}
	t.byFile[fileID] = activity
}

func (t *DownloadTracker) Decrement(fileID string, source string) {
	if t == nil || fileID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	activity, ok := t.byFile[fileID]
	if !ok {
		return
	}
	if activity.Total > 0 {
		activity.Total--
	}
	switch source {
	case "public":
		if activity.Public > 0 {
			activity.Public--
		}
	default:
		if activity.Auth > 0 {
			activity.Auth--
		}
	}
	if activity.Total == 0 {
		delete(t.byFile, fileID)
		return
	}
	t.byFile[fileID] = activity
}

func (t *DownloadTracker) Snapshot(fileID string) DownloadActivity {
	if t == nil || fileID == "" {
		return DownloadActivity{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	activity, ok := t.byFile[fileID]
	if !ok {
		return DownloadActivity{}
	}
	return activity
}
