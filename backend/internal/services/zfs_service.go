package services

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type ZfsService struct {
	cache sync.Map // map[string]zfsCacheItem
	sf    singleflight.Group
}

type zfsCacheItem struct {
	snapshot  *ZfsSnapshot
	expiresAt time.Time
}

func NewZfsService() *ZfsService {
	return &ZfsService{}
}

type ZfsSnapshot struct {
	Name     string
	Creation time.Time
}

// GetLatestSnapshot returns the latest snapshot for a given dataset path.
// It expects the path to be a mountpoint or part of a ZFS dataset.
// It runs: zfs list -H -t snapshot -o name,creation -s creation -r -d 1 <dataset_of_path>
// However, finding the dataset for a path can be tricky.
// A simpler first approach: `zfs list -H -o name <path>` to get dataset name, then list snapshots.
func (s *ZfsService) GetLatestSnapshot(ctx context.Context, path string) (*ZfsSnapshot, error) {
	// 1. Check Cache
	if val, ok := s.cache.Load(path); ok {
		item := val.(zfsCacheItem)
		// If valid, return
		if time.Now().Before(item.expiresAt) {
			return item.snapshot, nil
		}

		// If stale, return immediate but trigger background refresh (SWR)
		// We use singleflight in background to avoid spamming
		go func() {
			// Create a detached context with timeout for background work
			bgCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
			defer cancel()
			if _, err := s.refreshSnapshot(bgCtx, path); err != nil {
				slog.ErrorContext(bgCtx, "Failed to refresh ZFS snapshot in background", "path", path, "error", err)
			}
		}()
		return item.snapshot, nil
	}

	// 2. Cache Miss - Fetch Synchronously
	return s.refreshSnapshot(ctx, path)
}

// refreshSnapshot fetches the latest snapshot and updates the cache.
// It uses singleflight to ensure only one fetch per path happens at a time.
func (s *ZfsService) refreshSnapshot(ctx context.Context, path string) (*ZfsSnapshot, error) {
	// key for singleflight needs to be unique per path
	result, err, _ := s.sf.Do(path, func() (interface{}, error) {
		return s.fetchLatestSnapshot(ctx, path)
	})

	if err != nil {
		return nil, err
	}

	snapshot := result.(*ZfsSnapshot)

	// Update Cache (24h TTL)
	s.cache.Store(path, zfsCacheItem{
		snapshot:  snapshot,
		expiresAt: time.Now().Add(24 * time.Hour),
	})

	return snapshot, nil
}

// fetchLatestSnapshot contains the actual CLI logic
func (s *ZfsService) fetchLatestSnapshot(ctx context.Context, path string) (*ZfsSnapshot, error) {
	// Identify ZFS dataset by matching path against all mountpoints
	cmdList := exec.CommandContext(ctx, "zfs", "list", "-H", "-o", "name,mountpoint")
	var outList bytes.Buffer
	cmdList.Stdout = &outList
	if err := cmdList.Run(); err != nil {
		return nil, fmt.Errorf("failed to list zfs datasets: %w", err)
	}

	bestMatchLen := 0
	dataset := ""

	// Normalize path for comparison
	absPath, absErr := filepath.Abs(path)
	if absErr == nil {
		path = absPath
	}
	path = strings.TrimSuffix(path, string(os.PathSeparator))

	scanner := bufio.NewScanner(&outList)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		dsName := parts[0]
		mountpoint := parts[1]

		if mountpoint == "legacy" || mountpoint == "none" || mountpoint == "-" {
			continue
		}

		// Check if mountpoint is prefix of path
		// Handle potential trailing slash issues
		mpClean := strings.TrimSuffix(mountpoint, string(os.PathSeparator))

		if strings.HasPrefix(path, mpClean) {
			// Ensure it's a directory boundary (matches "/mnt/data" against "/mnt/data/sub" but not "/mnt/dataset2")
			rest := strings.TrimPrefix(path, mpClean)
			if rest == "" || strings.HasPrefix(rest, string(os.PathSeparator)) {
				if len(mpClean) > bestMatchLen {
					bestMatchLen = len(mpClean)
					dataset = dsName
				}
			}
		}
	}

	if dataset == "" {
		return nil, fmt.Errorf("no zfs dataset found mounting at or above %s", path)
	}

	// Now list snapshots for this dataset
	// zfs list -H -p -t snapshot -o name,creation -s creation -r -d 1 <dataset>
	cmdSnapP := exec.CommandContext(ctx, "zfs", "list", "-H", "-p", "-t", "snapshot", "-o", "name,creation", "-s", "creation", "-r", "-d", "1", dataset)
	var outSnapP bytes.Buffer
	cmdSnapP.Stdout = &outSnapP
	// Errors can be logged by caller if needed
	if err := cmdSnapP.Run(); err != nil {
		return nil, fmt.Errorf("failed to list snapshots for dataset %s: %w", dataset, err)
	}

	linesP := strings.Split(strings.TrimSpace(outSnapP.String()), "\n")
	if len(linesP) == 0 || linesP[0] == "" {
		// Dataset exists but no snapshots
		return nil, nil
	}

	lastLineP := linesP[len(linesP)-1]
	partsP := strings.Fields(lastLineP)
	if len(partsP) < 2 {
		// Should not happen with valid ZFS output
		return &ZfsSnapshot{Name: partsP[0]}, nil
	}

	timestamp := partsP[1]
	var ts int64
	fmt.Sscanf(timestamp, "%d", &ts)

	return &ZfsSnapshot{
		Name:     partsP[0],
		Creation: time.Unix(ts, 0),
	}, nil
}
