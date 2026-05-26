package analyze

import (
	"math"
	"sort"

	"github.com/IamWWT/dbexplain/schema"
)

// TableScore holds a deterministic importance score for a table.
type TableScore struct {
	Instance string             `json:"instance"`
	DB       string             `json:"db"`
	Table    string             `json:"table"`
	Score    float64            `json:"score"`
	Factors  map[string]float64 `json:"factors"`
}

// Ranker computes deterministic table importance scores.
// Zero AI inference — purely observable metrics.
type Ranker struct {
	Weights map[string]float64
}

// DefaultWeights returns the standard factor weights.
// Phase 2 factors (always available): graph_degree, fk_centrality, row_count, index_density
// Phase 3 factors (conditionally available): write_intensity, query_frequency
func DefaultWeights() map[string]float64 {
	return map[string]float64{
		"graph_degree":    0.30,
		"fk_centrality":   0.30,
		"row_count":       0.18,
		"index_density":   0.07,
		"write_intensity": 0.10,
		"query_frequency": 0.05,
	}
}

// NewRanker creates a Ranker with default weights.
func NewRanker() *Ranker {
	return &Ranker{Weights: DefaultWeights()}
}

// Rank computes importance scores for all tables in the universe.
func (r *Ranker) Rank(u *schema.Universe, refs []*schema.Ref) []TableScore {
	type key struct{ inst, db, table string }

	// Collect all tables
	var keys []key
	tableMap := map[key]*schema.Table{}
	rowCounts := map[key]int64{}
	indexDensities := map[key]float64{}

	for _, inst := range u.Instances {
		for _, db := range inst.Databases {
			for _, t := range db.Tables {
				k := key{inst.Label, db.Name, t.Name}
				keys = append(keys, k)
				tableMap[k] = t
				rowCounts[k] = t.RowCount
				if len(t.Columns) > 0 {
					indexDensities[k] = float64(len(t.Indexes)) / float64(len(t.Columns))
				}
			}
		}
	}

	if len(keys) == 0 {
		return nil
	}

	// Compute raw metrics
	graphDegree := map[key]int{}
	fkRefCount := map[key]int{}
	writeIntensity := map[key]float64{}
	queryFreq := map[key]float64{}

	for _, ref := range refs {
		from := key{ref.FromInstance, ref.FromDB, ref.FromTable}
		to := key{ref.ToInstance, ref.ToDB, ref.ToTable}
		graphDegree[from]++
		graphDegree[to]++
		fkRefCount[to]++
	}

	// Phase 3: operational stats
	for _, k := range keys {
		t := tableMap[k]
		if t.OpStats != nil {
			writeIntensity[k] = t.OpStats.WriteIntensity()
			// Query frequency: use query_count if available, else 0
			if t.OpStats.QueryCount > 0 {
				queryFreq[k] = logNorm(t.OpStats.QueryCount, 10000)
			}
		}
	}

	// Detect which Phase 3 factors have any data across the universe
	hasWrite := false
	hasQuery := false
	for _, k := range keys {
		if writeIntensity[k] > 0 {
			hasWrite = true
		}
		if queryFreq[k] > 0 {
			hasQuery = true
		}
	}

	// Build effective weights: zero out unavailable factors and re-normalize
	weights := make(map[string]float64)
	for k, v := range r.Weights {
		weights[k] = v
	}
	if !hasWrite {
		weights["write_intensity"] = 0
	}
	if !hasQuery {
		weights["query_frequency"] = 0
	}
	weightSum := 0.0
	for _, w := range weights {
		weightSum += w
	}

	// Find max values for normalization
	maxDegree := 1
	maxFK := 1
	var maxRows int64 = 1
	for _, k := range keys {
		if d := graphDegree[k]; d > maxDegree {
			maxDegree = d
		}
		if f := fkRefCount[k]; f > maxFK {
			maxFK = f
		}
		if rc := rowCounts[k]; rc > maxRows {
			maxRows = rc
		}
	}

	// Compute weighted scores
	var scores []TableScore
	for _, k := range keys {
		factors := map[string]float64{
			"graph_degree":    safeDiv(float64(graphDegree[k]), float64(maxDegree)),
			"fk_centrality":   safeDiv(float64(fkRefCount[k]), float64(maxFK)),
			"row_count":       logNorm(rowCounts[k], maxRows),
			"index_density":   clamp(indexDensities[k], 0, 1),
			"write_intensity": writeIntensity[k],
			"query_frequency": queryFreq[k],
		}

		var score float64
		for dim, weight := range weights {
			score += weight * factors[dim]
		}
		if weightSum > 0 {
			score = score / weightSum // re-normalize
		}
		score = math.Round(score*1000) / 1000 // round to 3 decimal places

		scores = append(scores, TableScore{
			Instance: k.inst,
			DB:       k.db,
			Table:    k.table,
			Score:    clamp(score, 0, 1),
			Factors:  factors,
		})
	}

	// Sort descending by score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	return scores
}

// safeDiv returns a/b, or 0 if b == 0.
func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// logNorm returns log10(v+1) / log10(max+1), clamped to [0,1].
// This compresses order-of-magnitude differences in row counts.
func logNorm(v, max int64) float64 {
	if max <= 0 {
		return 0
	}
	if v <= 0 {
		return 0
	}
	return math.Log10(float64(v)+1) / math.Log10(float64(max)+1)
}

// clamp constrains v to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
