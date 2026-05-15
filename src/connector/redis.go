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

func init() {
	Register("redis", func() Connector { return redisConnector{} })
}

type redisConnector struct{}

const (
	maxScanKeys   = 2000 // 扫描上限，足够发现模式，同时控制耗时
	scanBatchSize = 100  // 每批扫描数量
	hashSample    = 5    // hash 字段采样数
	streamSample  = 10   // stream 消息采样数
	getRangeLen   = 512  // string 值截取长度
)

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
		PoolSize:    10,
		MaxRetries:  1,
	})
	defer rdb.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, schema.NewDBError(d.Redacted(), "", "", "connect", err)
	}

	// 获取基础信息
	info, _ := rdb.Info(ctx, "server", "keyspace", "memory").Result()
	infoMap := parseRedisInfo(info)

	inst := &schema.Instance{DSN: d.Redacted(), Kind: "redis", Label: d.Label}
	dbEntry := &schema.Database{Name: fmt.Sprintf("db%d", dbIdx)}

	// ── 流式扫描并聚合 key 模式 ──
	logf(ctx, "[redis] start scanning keys...")
	families := streamScanAndGroup(ctx, rdb)
	logf(ctx, "[redis] scan finished, %d unique patterns", len(families))

	// ── 批量分析每个模式 ──
	for i, fam := range families {
		logf(ctx, "[redis] analyze pattern %d/%d: %s", i+1, len(families), fam.Pattern)
		t := buildFamilyTable(ctx, rdb, fam)
		dbEntry.Tables = append(dbEntry.Tables, t)
	}

	// 服务器概览
	summary := &schema.Table{
		Name: "_server_info",
		Comment: fmt.Sprintf("Redis %s | memory=%s | total_keys=%s",
			infoMap["redis_version"], infoMap["used_memory_human"],
			infoMap["db"+strconv.Itoa(dbIdx)]),
	}
	dbEntry.Tables = append([]*schema.Table{summary}, dbEntry.Tables...)
	inst.Databases = append(inst.Databases, dbEntry)
	return inst, nil
}

// ── 流式分组（边扫描边聚合，不存储全部 key） ──

type familyAgg struct {
	Pattern string
	Type    string // 未知，后续 pipeline 获取
	Count   int
	Example string
}

func streamScanAndGroup(ctx context.Context, rdb *redis.Client) []familyAgg {
	aggregates := map[string]*familyAgg{}
	iter := rdb.Scan(ctx, 0, "*", scanBatchSize).Iterator()
	totalScanned := 0
	for iter.Next(ctx) {
		key := iter.Val()
		pat := normalize(key)
		if agg, exists := aggregates[pat]; exists {
			agg.Count++
		} else {
			aggregates[pat] = &familyAgg{
				Pattern: pat,
				Count:   1,
				Example: key,
			}
		}
		totalScanned++
		if totalScanned%100 == 0 {
			logf(ctx, "[redis] scanned %d keys, %d patterns so far", totalScanned, len(aggregates))
		}
		if totalScanned >= maxScanKeys {
			break
		}
	}
	if err := iter.Err(); err != nil {
		logf(ctx, "[redis] scan error: %v", err)
	}

	// 按 key 数量排序
	fams := make([]familyAgg, 0, len(aggregates))
	for _, agg := range aggregates {
		fams = append(fams, *agg)
	}
	sort.Slice(fams, func(i, j int) bool { return fams[i].Count > fams[j].Count })
	return fams
}

// ── 构建表信息（含 pipeline 批量检测与风险标记） ──

func buildFamilyTable(ctx context.Context, rdb *redis.Client, fam familyAgg) *schema.Table {
	t := &schema.Table{
		Name:       fam.Pattern,
		KeyPattern: fam.Pattern,
		RowCount:   int64(fam.Count),
		DataType:   fam.Type,
	}

	if fam.Example == "" {
		t.Comment = "no keys sampled"
		return t
	}

	// Pipeline：一次性获取 TYPE, TTL, MEMORY USAGE
	pipe := rdb.Pipeline()
	typeCmd := pipe.Type(ctx, fam.Example)
	ttlCmd := pipe.TTL(ctx, fam.Example)
	memCmd := pipe.MemoryUsage(ctx, fam.Example, 0) // 0 表示不采样
	_, err := pipe.Exec(ctx)
	if err != nil {
		logf(ctx, "[redis] pipeline failed for %s: %v", fam.Example, err)
		t.Comment = fmt.Sprintf("read error: %v", err)
		return t
	}

	ktype, _ := typeCmd.Result()
	ttlDuration, _ := ttlCmd.Result()
	memBytes, _ := memCmd.Result()

	t.DataType = ktype

	// 根据类型获取字段信息（安全限制）
	var risks []string
	switch ktype {
	case "hash":
		fields := sampleHash(ctx, rdb, fam.Example, hashSample)
		for f, v := range fields {
			c := &schema.Column{Name: f, Type: inferType(v), Comment: "hash field"}
			t.Columns = append(t.Columns, c)
		}
		if len(fields) > 0 {
			hlen, _ := rdb.HLen(ctx, fam.Example).Result()
			if hlen > 1000 {
				risks = append(risks, fmt.Sprintf("large hash (%d fields)", hlen))
			}
		}

	case "string":
		val := sampleString(ctx, rdb, fam.Example, getRangeLen)
		c := &schema.Column{Name: "(value)", Type: inferType(val), Comment: truncate(val, 60)}
		t.Columns = append(t.Columns, c)
		if len(val) > 1*1024*1024 {
			risks = append(risks, fmt.Sprintf("large string (%d bytes)", len(val)))
		}
		if ttlDuration > 0 {
			t.Columns = append(t.Columns, &schema.Column{
				Name: "ttl", Type: "duration", Comment: ttlDuration.String(),
			})
		}

	case "zset":
		t.Columns = []*schema.Column{
			{Name: "member", Type: "string"},
			{Name: "score", Type: "float64"},
		}
		zcard, _ := rdb.ZCard(ctx, fam.Example).Result()
		if zcard > 10000 {
			risks = append(risks, fmt.Sprintf("large sorted set (%d members)", zcard))
		}

	case "list":
		t.Columns = []*schema.Column{{Name: "element", Type: "string"}}
		llen, _ := rdb.LLen(ctx, fam.Example).Result()
		if llen > 10000 {
			risks = append(risks, fmt.Sprintf("long list (%d elements)", llen))
		}

	case "set":
		t.Columns = []*schema.Column{{Name: "member", Type: "string"}}
		scard, _ := rdb.SCard(ctx, fam.Example).Result()
		if scard > 10000 {
			risks = append(risks, fmt.Sprintf("large set (%d members)", scard))
		}
		
	case "stream":
		fields := sampleStream(ctx, rdb, fam.Example, streamSample)
		for f, typ := range fields {
			t.Columns = append(t.Columns, &schema.Column{
				Name: f, Type: typ, Comment: "stream field",
			})
		}
		// 检查未消费消息
		groups, err := rdb.XInfoGroups(ctx, fam.Example).Result()
		if err == nil && len(groups) == 0 {
			if xlen, err := rdb.XLen(ctx, fam.Example).Result(); err == nil && xlen > 1000 {
				risks = append(risks, fmt.Sprintf("stream without consumer group (%d pending)", xlen))
			}
		}
	}
	

	// 通用风险：无 TTL 但 key 名称暗示应有过期
	if ttlDuration <= 0 && isSecuritySensitive(fam.Pattern) {
		risks = append(risks, "no TTL on security‑sensitive key")
	}
	if ttlDuration > 0 && ttlDuration > 30*24*time.Hour {
		risks = append(risks, fmt.Sprintf("very long TTL (%s)", ttlDuration))
	}

	// 将风险写入注释
	comment := fmt.Sprintf("%d keys, type=%s", fam.Count, ktype)
	if memBytes > 0 {
		comment += fmt.Sprintf(", avg memory=%.1fKB", float64(memBytes)/1024)
	}
	if len(risks) > 0 {
		comment += " | ⚠️ " + strings.Join(risks, "; ")
	}
	t.Comment = comment

	return t
}

// ── 安全采样函数 ──

func sampleHash(ctx context.Context, rdb *redis.Client, key string, count int) map[string]string {
	result := make(map[string]string)
	iter := rdb.HScan(ctx, key, 0, "*", int64(count)).Iterator()
	for iter.Next(ctx) {
		field := iter.Val()
		if !iter.Next(ctx) {
			break
		}
		val := iter.Val()
		result[field] = val
	}
	return result
}

func sampleString(ctx context.Context, rdb *redis.Client, key string, maxLen int) string {
	val, err := rdb.GetRange(ctx, key, 0, int64(maxLen-1)).Result()
	if err != nil {
		return ""
	}
	return val
}

func sampleStream(ctx context.Context, rdb *redis.Client, key string, count int) map[string]string {
	fields := make(map[string]string)
	msgs, err := rdb.XRangeN(ctx, key, "-", "+", int64(count)).Result()
	if err != nil {
		return fields
	}
	for _, msg := range msgs {
		for f, v := range msg.Values {
			if _, exists := fields[f]; !exists {
				fields[f] = inferType(fmt.Sprintf("%v", v))
			}
		}
	}
	return fields
}

// ── 工具函数（保留原有逻辑） ──

var numRe = regexp.MustCompile(`\d{2,}`)
var hexRe = regexp.MustCompile(`[0-9a-fA-F]{8,}`)
var uuidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func normalize(k string) string {
	k = uuidRe.ReplaceAllString(k, "{uuid}")
	k = hexRe.ReplaceAllString(k, "{hex}")
	k = numRe.ReplaceAllString(k, "{id}")
	return k
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

func isSecuritySensitive(pattern string) bool {
	sensitive := []string{"session", "token", "auth", "otp", "captcha", "login", "credential"}
	lower := strings.ToLower(pattern)
	for _, s := range sensitive {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
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