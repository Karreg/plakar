package tui

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/PlakarKorp/kloset/location"
	"github.com/PlakarKorp/kloset/objects"
	"github.com/PlakarKorp/kloset/repository"
	"github.com/PlakarKorp/kloset/snapshot"
)

type DashboardData struct {
	Location        string
	Total           int
	StorageSize     int64
	LogicalSize     int64
	Efficiency      float64
	SnapshotsPerDay []int
}

type SnapshotInfo struct {
	ID        objects.MAC
	ShortID   string
	Timestamp time.Time
	Size      uint64
	Duration  time.Duration
	Importer  string
	Tags      []string
}

type EntryInfo struct {
	Name          string
	Path          string
	IsDir         bool
	Size          int64
	Mode          fs.FileMode
	ModTime       time.Time
	SymlinkTarget string
	ContentType   string
	Chunks        uint64
}

func LoadDashboard(repo *repository.Repository) (*DashboardData, error) {
	total, logicalSize, err := snapshot.LogicalSize(repo)
	if err != nil {
		return nil, fmt.Errorf("calculate logical size: %w", err)
	}
	storageSize, err := repo.StorageSize()
	if err != nil {
		return nil, fmt.Errorf("calculate storage size: %w", err)
	}

	const days = 30
	snapshotsPerDay := make([]int, days)
	cutoff := repo.Configuration().Timestamp.AddDate(0, 0, -days)
	for snapshotID, err := range repo.ListSnapshots() {
		if err != nil {
			continue
		}
		snap, err := snapshot.Load(repo, snapshotID)
		if err != nil {
			continue
		}
		timestamp := snap.Header.Timestamp
		if !timestamp.Before(cutoff) {
			dayIndex := time.Since(timestamp).Hours() / 24
			if dayIndex >= 0 && dayIndex < days {
				snapshotsPerDay[(days-1)-int(dayIndex)]++
			}
		}
		_ = snap.Close()
	}

	efficiency := float64(0)
	if storageSize == -1 || logicalSize == 0 {
		efficiency = -1
	} else {
		usagePercent := float64(storageSize) / float64(logicalSize) * 100
		if usagePercent <= 100 {
			efficiency = 100 - usagePercent
		} else if increase := usagePercent - 100; increase <= 100 {
			efficiency = -increase
		} else {
			efficiency = -1
		}
	}

	locationString := repo.Type() + "://"
	flags := repo.Store().Flags()
	if origin := repo.Origin(); origin != "" && flags&location.FLAG_LOCALFS == 0 {
		locationString += origin
	}
	if root := repo.Root(); root != "" && root != "/" {
		locationString += root
	}

	return &DashboardData{
		Location:        locationString,
		Total:           total,
		StorageSize:     storageSize,
		LogicalSize:     logicalSize,
		Efficiency:      efficiency,
		SnapshotsPerDay: snapshotsPerDay,
	}, nil
}

func ListSnapshots(repo *repository.Repository) ([]SnapshotInfo, error) {
	snapshots := make([]SnapshotInfo, 0)
	for snapshotID, err := range repo.ListSnapshots() {
		if err != nil {
			return nil, fmt.Errorf("list snapshots: %w", err)
		}
		snap, err := snapshot.Load(repo, snapshotID)
		if err != nil {
			return nil, fmt.Errorf("load snapshot %s: %w", hex.EncodeToString(snapshotID[:]), err)
		}
		source := snap.Header.GetSource(0)
		snapshots = append(snapshots, SnapshotInfo{
			ID:        snapshotID,
			ShortID:   hex.EncodeToString(snap.Header.GetIndexShortID()),
			Timestamp: snap.Header.Timestamp,
			Size:      source.Summary.Directory.Size + source.Summary.Below.Size,
			Duration:  snap.Header.Duration,
			Importer:  source.Importer.Directory,
			Tags:      append([]string(nil), snap.Header.Tags...),
		})
		if err := snap.Close(); err != nil {
			return nil, fmt.Errorf("close snapshot %s: %w", hex.EncodeToString(snapshotID[:]), err)
		}
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})
	return snapshots, nil
}

func ListDir(repo *repository.Repository, snapshotID objects.MAC, dirPath string) ([]EntryInfo, error) {
	snap, err := snapshot.Load(repo, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("load snapshot %s: %w", hex.EncodeToString(snapshotID[:]), err)
	}
	defer snap.Close()

	filesystem, err := snap.Filesystem()
	if err != nil {
		return nil, fmt.Errorf("open snapshot filesystem: %w", err)
	}
	if dirPath == "" {
		dirPath = snap.Header.GetSource(0).Importer.Directory
	}
	entry, err := filesystem.GetEntry(path.Clean(dirPath))
	if err != nil {
		return nil, fmt.Errorf("get directory %q: %w", dirPath, err)
	}
	if !entry.IsDir() {
		return nil, fmt.Errorf("%q is not a directory", dirPath)
	}
	children, err := entry.Getdents(filesystem)
	if err != nil {
		return nil, fmt.Errorf("list directory %q: %w", dirPath, err)
	}

	entries := make([]EntryInfo, 0)
	for child, err := range children {
		if err != nil {
			return nil, fmt.Errorf("read directory %q: %w", dirPath, err)
		}
		stat := child.Stat()
		entries = append(entries, EntryInfo{
			Name:          child.Name(),
			Path:          child.Path(),
			IsDir:         child.IsDir(),
			Size:          stat.Lsize,
			Mode:          stat.Lmode,
			ModTime:       stat.LmodTime,
			SymlinkTarget: child.SymlinkTarget,
			ContentType:   child.ContentType,
			Chunks:        child.Chunks,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

type snapshotFile struct {
	io.ReadCloser
	snapshot *snapshot.Snapshot
}

func (f *snapshotFile) Close() error {
	readErr := f.ReadCloser.Close()
	snapshotErr := f.snapshot.Close()
	if readErr != nil {
		return readErr
	}
	return snapshotErr
}

func OpenFile(repo *repository.Repository, snapshotID objects.MAC, filePath string) (io.ReadCloser, *EntryInfo, error) {
	snap, err := snapshot.Load(repo, snapshotID)
	if err != nil {
		return nil, nil, fmt.Errorf("load snapshot %s: %w", hex.EncodeToString(snapshotID[:]), err)
	}

	filesystem, err := snap.Filesystem()
	if err != nil {
		_ = snap.Close()
		return nil, nil, fmt.Errorf("open snapshot filesystem: %w", err)
	}
	entry, err := filesystem.GetEntry(path.Clean(filePath))
	if err != nil {
		_ = snap.Close()
		return nil, nil, fmt.Errorf("get file %q: %w", filePath, err)
	}
	stat := entry.Stat()
	if !stat.Lmode.IsRegular() {
		_ = snap.Close()
		return nil, nil, fmt.Errorf("%q is not a regular file", filePath)
	}
	reader, err := entry.Open(filesystem)
	if err != nil {
		_ = snap.Close()
		return nil, nil, fmt.Errorf("open file %q: %w", filePath, err)
	}
	info := &EntryInfo{
		Name:        entry.Name(),
		Path:        entry.Path(),
		Size:        stat.Lsize,
		Mode:        stat.Lmode,
		ModTime:     stat.LmodTime,
		ContentType: entry.ContentType,
		Chunks:      entry.Chunks,
	}
	return &snapshotFile{ReadCloser: reader, snapshot: snap}, info, nil
}
