package connector

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"dbexplain/dsn"
	"dbexplain/schema"
)

type redisConnector struct{}

func (redisConnector) Collect(ctx context.Context, d *dsn.DSN) (*schema.Instance, error) {
	host := d.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := d.Port
	if port == "" {
		port = "6379"
	}
	dbIdx := 0
	if d.DBName != "" {
		dbIdx, _ = strconv.Atoi(d.DBName)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:        fmt.Sprintf("%s:%s", host, port),
		Password:    d.Password,
		DB:          dbIdx,
		DialTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second,
	})
	defer rdb.Close()

	// Ping 使用外部上下文
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis connect: %w", err)
	}

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "redis", Label: d.Label}

	info, _ := rdb.Info(ctx, "server", "keyspace", "memory").Result()
	infoMap := parseRedisInfo(info)

	dbEntry := &schema.Database{Name: fmt.Sprintf("db%d", dbIdx)}

	// scan keys
	var keys []string
	var cursor uint64
	for len(keys) < 500 {
		batch, c, err := rdb.Scan(ctx, cursor, "*", 100).Result()
		if err != nil {
			break
		}
		keys = append(keys, batch...)
		cursor = c
		if cursor == 0 {
			break
		}
	}

	families := groupKeys(keys)
	for _, fam := range families {
		t := &schema.Table{
			Name:       fam.Pattern,
			KeyPattern: fam.Pattern,
			RowCount:   int64(fam.Count),
			Comment:    fmt.Sprintf("%d keys, type=%s", fam.Count, fam.Type),
			DataType:   fam.Type,
		}
		if fam.Example != "" {
			inspectRedisKey(ctx, rdb, fam.Example, fam.Type, t)
		}
		dbEntry.Tables = append(dbEntry.Tables, t)
	}

	summary := &schema.Table{
		Name:    "_server_info",
		Comment: fmt.Sprintf("Redis %s | memory=%s | total_keys=%s", infoMap["redis_version"], infoMap["used_memory_human"], infoMap["db"+strconv.Itoa(dbIdx)]),
	}
	dbEntry.Tables = append([]*schema.Table{summary}, dbEntry.Tables...)
	inst.Databases = append(inst.Databases, dbEntry)
	return inst, nil
}

type keyFamily struct {
	Pattern string
	Type    string
	Count   int
	Example string
}

var numRe = regexp.MustCompile(`\d{2,}`)
var hexRe = regexp.MustCompile(`[0-9a-fA-F]{8,}`)
var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func normalize(k string) string {
	k = uuidRe.ReplaceAllString(k, "{uuid}")
	k = hexRe.ReplaceAllString(k, "{hex}")
	k = numRe.ReplaceAllString(k, "{id}")
	return k
}

func groupKeys(keys []string) []keyFamily {
	type entry struct {
		count   int
		example string
	}
	m := map[string]*entry{}
	for _, k := range keys {
		pat := normalize(k)
		if e, ok := m[pat]; ok {
			e.count++
		} else {
			m[pat] = &entry{count: 1, example: k}
		}
	}
	var fams []keyFamily
	for pat, e := range m {
		fams = append(fams, keyFamily{Pattern: pat, Count: e.count, Example: e.example})
	}
	sort.Slice(fams, func(i, j int) bool { return fams[i].Count > fams[j].Count })
	return fams
}

func inspectRedisKey(ctx context.Context, rdb *redis.Client, key, ktype string, t *schema.Table) {
	if ktype == "" {
		ktype, _ = rdb.Type(ctx, key).Result()
		t.DataType = ktype
	}
	switch ktype {
	case "hash":
		fields, err := rdb.HGetAll(ctx, key).Result()
		if err != nil {
			return
		}
		for f, v := range fields {
			c := &schema.Column{Name: f, Type: inferType(v), Comment: "hash field"}
			t.Columns = append(t.Columns, c)
		}
	case "string":
		v, err := rdb.Get(ctx, key).Result()
		if err != nil {
			return
		}
		t.Columns = append(t.Columns, &schema.Column{
			Name: "(value)", Type: inferType(v),
			Comment: truncate(v, 60),
		})
		ttl, _ := rdb.TTL(ctx, key).Result()
		if ttl > 0 {
			t.Columns = append(t.Columns, &schema.Column{Name: "ttl", Type: "duration", Comment: ttl.String()})
		}
	case "zset":
		t.Columns = []*schema.Column{
			{Name: "member", Type: "string"},
			{Name: "score", Type: "float64"},
		}
	case "list":
		t.Columns = []*schema.Column{{Name: "element", Type: "string"}}
	case "set":
		t.Columns = []*schema.Column{{Name: "member", Type: "string"}}
	case "stream":
		msgs, err := rdb.XRange(ctx, key, "-", "+").Result()
		if err != nil || len(msgs) == 0 {
			return
		}
		seen := map[string]bool{}
		for _, msg := range msgs {
			for f, v := range msg.Values {
				if !seen[f] {
					t.Columns = append(t.Columns, &schema.Column{
						Name: f, Type: inferType(fmt.Sprintf("%v", v)), Comment: "stream field",
					})
					seen[f] = true
				}
			}
		}
	}
}

func inferType(v string) string {
	if _, err := strconv.ParseInt(v, 10, 64); err == nil {
		return "integer"
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return "float"
	}
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[") {
		return "json"
	}
	return "string"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func parseRedisInfo(info string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			m[parts[0]] = strings.TrimSpace(parts[1])
		}
	}
	return m
}