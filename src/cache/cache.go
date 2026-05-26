// Package cache provides schema fingerprinting and delta scan support.
//
// Each table's schema is hashed into a fingerprint that can be compared
// across runs. When two consecutive scans produce the same fingerprint,
// the table's metadata hasn't changed and downstream processing can be
// skipped.
//
// This is critical for enterprise databases where full scans are impractical.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/IamWWT/dbexplain/schema"
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

// ── Disk-backed fingerprint store ──

// Store persists and compares fingerprints across runs.
type Store struct {
	path    string
	entries map[string]Fingerprint
}

// LoadStore loads a fingerprint store from disk, or creates a new one.
func LoadStore(path string) (*Store, error) {
	s := &Store{
		path:    path,
		entries: make(map[string]Fingerprint),
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	var fps []Fingerprint
	if err := json.Unmarshal(data, &fps); err != nil {
		return s, fmt.Errorf("invalid fingerprint cache: %w", err)
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

// Update stores fingerprints for all tables in the universe and saves to disk.
func (s *Store) Update(u *schema.Universe) error {
	fps := ComputeUniverse(u)
	for _, fp := range fps {
		s.entries[fp.key()] = fp
	}
	return s.Save()
}

// Save persists the current fingerprint store to disk.
func (s *Store) Save() error {
	var fps []Fingerprint
	for _, fp := range s.entries {
		fps = append(fps, fp)
	}
	data, err := json.MarshalIndent(fps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
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
