// Package cache provides schema fingerprinting, delta scan support,
// and snapshot storage for field-level diff analysis.
//
// Each table's schema is hashed into a fingerprint that can be compared
// across runs. When two consecutive scans produce the same fingerprint,
// the table's metadata hasn't changed and downstream processing can be
// skipped.
//
// Snapshots store the full Table metadata alongside fingerprints, enabling
// field-level diff output via the internal/diff package.
//
// This is critical for enterprise databases where full scans are impractical.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/internal/diff"
	"github.com/IamWWT/dbexplain/internal/schema"
)

// Fingerprint is a deterministically computed signature of a table's schema.
// Two tables with the same fingerprint have identical structural metadata.
type Fingerprint struct {
	Instance  string    `json:"instance"`
	DB        string    `json:"db"`
	Table     string    `json:"table"`
	ColHash   string    `json:"col_hash"`
	IndexHash string    `json:"index_hash"`
	FKHash    string    `json:"fk_hash"`
	RowCount  int64     `json:"row_count,omitempty"`
	ScannedAt time.Time `json:"scanned_at"`
}

// key returns the lookup key for this fingerprint.
func (f Fingerprint) key() string {
	return f.Instance + "/" + f.DB + "/" + f.Table
}

// Equal returns true if two fingerprints have identical hashes.
func (f Fingerprint) Equal(other Fingerprint) bool {
	return f.ColHash == other.ColHash &&
		f.IndexHash == other.IndexHash &&
		f.FKHash == other.FKHash
}

// ComputeFingerprint creates a fingerprint from a schema table.
func ComputeFingerprint(instance, db string, t *schema.Table) Fingerprint {
	return Fingerprint{
		Instance:  instance,
		DB:        db,
		Table:     t.Name,
		ColHash:   hashColumns(t.Columns),
		IndexHash: hashIndexes(t.Indexes),
		FKHash:    hashFKs(t.ForeignKeys),
		RowCount:  t.RowCount,
		ScannedAt: time.Now(),
	}
}

// ComputeUniverse computes fingerprints for all tables in the universe.
func ComputeUniverse(u *schema.Universe) []Fingerprint {
	var fps []Fingerprint
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				fps = append(fps, ComputeFingerprint(inst.Label, db.Name, t))
			}
		}
	}
	return fps
}

// ── Hashing helpers ──

func hashColumns(cols []*schema.Column) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = fmt.Sprintf("%s:%s:%v", strings.ToLower(c.Name), strings.ToLower(c.Type), c.Nullable)
	}
	sort.Strings(parts)
	return sha256Hex(strings.Join(parts, "|"))
}

func hashIndexes(indexes []*schema.Index) string {
	parts := make([]string, len(indexes))
	for i, idx := range indexes {
		cols := make([]string, len(idx.Columns))
		copy(cols, idx.Columns)
		sort.Strings(cols)
		parts[i] = fmt.Sprintf("%s:%v:%s", strings.ToLower(idx.Name), idx.Unique, strings.Join(cols, ","))
	}
	sort.Strings(parts)
	return sha256Hex(strings.Join(parts, "|"))
}

func hashFKs(fks []*schema.ForeignKey) string {
	parts := make([]string, len(fks))
	for i, fk := range fks {
		fromCols := make([]string, len(fk.Columns))
		copy(fromCols, fk.Columns)
		sort.Strings(fromCols)
		refCols := make([]string, len(fk.RefColumns))
		copy(refCols, fk.RefColumns)
		sort.Strings(refCols)
		parts[i] = fmt.Sprintf("%s:%s:%s:%s",
			strings.Join(fromCols, ","),
			fk.RefTable,
			strings.Join(refCols, ","),
			strings.ToLower(fk.Name),
		)
	}
	sort.Strings(parts)
	return sha256Hex(strings.Join(parts, "|"))
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ── Disk-backed store (fingerprints + snapshots) ──

// VersionEntry stores a complete snapshot of fingerprints + table metadata
// for one version, enabling --since cross-version comparisons.
type VersionEntry struct {
	Fingerprints []Fingerprint            `json:"fingerprints"`
	Snapshots    map[string]*schema.Table `json:"snapshots,omitempty"`
	CreatedAt    time.Time                `json:"created_at"`
}

// storeFile is the JSON file structure for cache persistence.
// Supports backward compatibility with v0.1.0 format (plain array of fingerprints).
type storeFile struct {
	Fingerprints []Fingerprint              `json:"fingerprints"`
	Snapshots    map[string]*schema.Table   `json:"snapshots,omitempty"`
	Versions     map[string]VersionEntry    `json:"versions,omitempty"`
}

// Store persists fingerprints, snapshots, and version history across runs.
type Store struct {
	path      string
	entries   map[string]Fingerprint
	snapshots map[string]*schema.Table // key = "instance/db/table"
	versions  map[string]VersionEntry  // labeled historical versions
}

// LoadStore loads a store from disk, or creates a new one.
// Supports both the v0.1.1+ format ({"fingerprints": [...], "snapshots": {...}})
// and the v0.1.0 format (plain array of fingerprints).
func LoadStore(path string) (*Store, error) {
	s := &Store{
		path:      path,
		entries:   make(map[string]Fingerprint),
		snapshots: make(map[string]*schema.Table),
		versions:  make(map[string]VersionEntry),
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}

	// Try new format (object with fingerprints+snapshots+versions)
	var sf storeFile
	if err := json.Unmarshal(data, &sf); err == nil {
		for _, fp := range sf.Fingerprints {
			s.entries[fp.key()] = fp
		}
		if sf.Snapshots != nil {
			s.snapshots = sf.Snapshots
		}
		if sf.Versions != nil {
			s.versions = sf.Versions
		}
		return s, nil
	}

	// Fall back to v0.1.0 format (plain array)
	var fps []Fingerprint
	if err := json.Unmarshal(data, &fps); err != nil {
		return s, fmt.Errorf("invalid cache file (tried new and old format): %w", err)
	}
	for _, fp := range fps {
		s.entries[fp.key()] = fp
	}
	return s, nil
}

// HasChanged returns true if a table's schema has changed since the last scan.
// Returns true for new tables (no previous fingerprint).
func (s *Store) HasChanged(instance, db string, t *schema.Table) bool {
	prev, ok := s.entries[instance+"/"+db+"/"+t.Name]
	if !ok {
		return true // new table
	}
	current := ComputeFingerprint(instance, db, t)
	return !current.Equal(prev)
}

// Update stores fingerprints and snapshots for all tables and saves to disk.
func (s *Store) Update(u *schema.Universe) error {
	fps := ComputeUniverse(u)
	for _, fp := range fps {
		s.entries[fp.key()] = fp
	}
	// Store snapshots for all tables
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				key := inst.Label + "/" + db.Name + "/" + t.Name
				s.snapshots[key] = t
			}
		}
	}
	return s.Save()
}

// Save persists the store to disk atomically.
func (s *Store) Save() error {
	sf := storeFile{
		Fingerprints: make([]Fingerprint, 0, len(s.entries)),
		Snapshots:    s.snapshots,
		Versions:     s.versions,
	}
	for _, fp := range s.entries {
		sf.Fingerprints = append(sf.Fingerprints, fp)
	}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: temp file + rename (atomic on POSIX, same filesystem)
	// On Windows, os.Rename fails if target exists — remove first.
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		os.Remove(s.path)
	}
	return os.Rename(tmpPath, s.path)
}

// GetSnapshots returns the current snapshot map.
func (s *Store) GetSnapshots() map[string]*schema.Table {
	return s.snapshots
}

// ── Version management (P4 multi-version baseline) ──

// SaveVersion saves the current state as a named version and saves to disk.
func (s *Store) SaveVersion(label string) error {
	fps := make([]Fingerprint, 0, len(s.entries))
	for _, fp := range s.entries {
		fps = append(fps, fp)
	}
	snaps := make(map[string]*schema.Table)
	for k, v := range s.snapshots {
		snaps[k] = v
	}
	s.versions[label] = VersionEntry{
		Fingerprints: fps,
		Snapshots:    snaps,
		CreatedAt:    time.Now(),
	}
	return s.Save()
}

// LoadVersion retrieves fingerprints and snapshots for a named version.
func (s *Store) LoadVersion(label string) (map[string]Fingerprint, map[string]*schema.Table, error) {
	ve, ok := s.versions[label]
	if !ok {
		return nil, nil, fmt.Errorf("version %q not found", label)
	}
	fps := make(map[string]Fingerprint)
	for _, fp := range ve.Fingerprints {
		fps[fp.key()] = fp
	}
	return fps, ve.Snapshots, nil
}

// ListVersions returns sorted version labels.
func (s *Store) ListVersions() []string {
	labels := make([]string, 0, len(s.versions))
	for l := range s.versions {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	return labels
}

// DiffSince compares the store's current state against a named historical version.
func (s *Store) DiffSince(versionLabel string, instanceFn, dbFn func(string) string) (diff.DiffResult, error) {
	oldFPs, oldSnaps, err := s.LoadVersion(versionLabel)
	if err != nil {
		return diff.DiffResult{}, err
	}

	oldKeys := make(map[string]bool)
	for k := range oldFPs {
		oldKeys[k] = true
	}

	currentKeys := make(map[string]bool)
	for k := range s.entries {
		currentKeys[k] = true
	}

	return diff.DiffUniverse(
		oldKeys, currentKeys,
		oldSnaps, s.snapshots,
		instanceFn, dbFn,
	), nil
}

// ── Delta computation ──

// Delta describes what changed between two scans.
type Delta struct {
	Added    []string `json:"added"`
	Removed  []string `json:"removed"`
	Changed  []string `json:"changed"`
}

// Diff compares current fingerprints against the store and returns the delta.
func (s *Store) Diff(u *schema.Universe) Delta {
	current := ComputeUniverse(u)
	currentKeys := make(map[string]Fingerprint)
	for _, fp := range current {
		currentKeys[fp.key()] = fp
	}

	var d Delta

	// Added / Changed
	for _, fp := range current {
		prev, ok := s.entries[fp.key()]
		if !ok {
			d.Added = append(d.Added, fp.key())
		} else if !fp.Equal(prev) {
			d.Changed = append(d.Changed, fp.key())
		}
	}

	// Removed
	for key := range s.entries {
		if _, ok := currentKeys[key]; !ok {
			d.Removed = append(d.Removed, key)
		}
	}

	return d
}

// DiffDetailed compares current universe against stored snapshots and returns
// field-level diff results. Requires snapshots to be available; falls back to
// Delta-level reporting if snapshots are missing.
func (s *Store) DiffDetailed(u *schema.Universe) diff.DiffResult {
	current := ComputeUniverse(u)
	currentKeys := make(map[string]bool)
	for _, fp := range current {
		currentKeys[fp.key()] = true
	}

	oldKeys := make(map[string]bool)
	for _, fp := range s.entries {
		oldKeys[fp.key()] = true
	}

	return diff.DiffUniverse(
		oldKeys, currentKeys,
		s.snapshots, buildSnapshots(u),
		diff.NewInstanceLabelFunc(),
		diff.NewDBNameFunc(),
	)
}

// buildSnapshots builds a snapshot map from a universe.
func buildSnapshots(u *schema.Universe) map[string]*schema.Table {
	snaps := make(map[string]*schema.Table)
	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				key := inst.Label + "/" + db.Name + "/" + t.Name
				snaps[key] = t
			}
		}
	}
	return snaps
}
