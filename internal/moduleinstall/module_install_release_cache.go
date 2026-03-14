package moduleinstall

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	releaseMetadataCacheTTL = time.Minute
	releaseHTTPClient       = http.DefaultClient

	releaseMetadataCacheMu sync.Mutex
	releaseMetadataCache   = map[string]releaseMetadataCacheEntry{}
)

type releaseMetadataCacheEntry struct {
	Value     string
	ExpiresAt time.Time
}

func loadReleaseMetadataCache(key string) (string, bool) {
	releaseMetadataCacheMu.Lock()
	defer releaseMetadataCacheMu.Unlock()

	entry, ok := releaseMetadataCache[strings.TrimSpace(key)]
	if !ok {
		return "", false
	}
	if !entry.ExpiresAt.IsZero() && time.Now().UTC().After(entry.ExpiresAt) {
		delete(releaseMetadataCache, strings.TrimSpace(key))
		return "", false
	}
	return entry.Value, true
}

func storeReleaseMetadataCache(key string, value string) {
	releaseMetadataCacheMu.Lock()
	defer releaseMetadataCacheMu.Unlock()

	releaseMetadataCache[strings.TrimSpace(key)] = releaseMetadataCacheEntry{
		Value:     strings.TrimSpace(value),
		ExpiresAt: time.Now().UTC().Add(releaseMetadataCacheTTL),
	}
}
