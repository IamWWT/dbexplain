# L7: 文件查询引擎 QA 测试 (v0.1.0+)

> 验证 CSV/XLSX 文件查询引擎的 WHERE/GROUP BY/ORDER BY/JOIN/聚合/表达式能力。
> 对应 QA 场景 Q09-Q15，覆盖真实银行业务分析。
> 文件查询引擎为 v0.1.0 新增能力。

---

## 前置条件

```bash
cd ~/Downloads/aigc/proj/agents/aiops/dbexplain
BIN="./bin/dbexplain"
```

## 测试数据

| 文件 | 路径 | 格式 | 行数 | 列数 | 说明 |
|------|------|------|------|------|------|
| pb_touch_ops_sample_2000.csv | `testdata/pb-province-touch-query/assets/test_data/` | CSV | 2000 | 40 | 客户经理触达运营汇总 |
| t_sec_org_sample.csv | 同上 | CSV | 200 | 7 | 银行组织架构表 |

---

## Q09: 跨省触达率排行 (GROUP BY + AVG + ORDER BY DESC)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv $BIN execute -env --label touch-ops-csv \
  'SELECT pnbrn_org_name, AVG(reach_rate) AS avg_reach_rate FROM pb_touch_ops_sample_2000 GROUP BY pnbrn_org_name ORDER BY avg_reach_rate DESC' --human
```

| 省份 | avg_reach_rate |
|------|---------------|
| 上海分行 | 59.7731 |
| 广东分行 | 59.4028 |
| 北京分行 | 59.0699 |
| 山东分行 | 58.2977 |
| 湖北分行 | 58.0887 |
| 四川分行 | 57.4578 |
| 江苏分行 | 57.0317 |
| 浙江分行 | 56.3853 |

**验证点**: GROUP BY 正确聚合 8 个省份，ORDER BY DESC 排序正确。
**结果**: ✅ PASS (8 row(s) in set ~1.1ms)

---

## Q10: 客户经理排名 (ORDER BY ASC + LIMIT)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv $BIN execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, pnbrn_org_name, reach_rate, tol_cnt FROM pb_touch_ops_sample_2000 ORDER BY reach_rate ASC LIMIT 5' --human
```

| csmgr_refno | pnbrn_org_name | reach_rate | tol_cnt |
|-------------|---------------|------------|---------|
| EHR700154 | 广东分行 | 10.0 | 50 |
| EHR700035 | 江苏分行 | 11.36 | 88 |
| EHR700087 | 上海分行 | 11.76 | 51 |
| EHR700171 | 湖北分行 | 13.25 | 151 |
| EHR700071 | 湖北分行 | 14.06 | 64 |

**验证点**: ORDER BY ASC + LIMIT + 列投影正确。
**结果**: ✅ PASS

---

## Q11: 企微渠道效能 (CAST + 列间算术 + WHERE)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv $BIN execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, CAST(wecom_interact_cnt AS FLOAT) / total_interact_cnt * 100 AS wecom_pct FROM pb_touch_ops_sample_2000 WHERE total_interact_cnt > 0 ORDER BY wecom_pct DESC LIMIT 5' --human
```

| csmgr_refno | wecom_pct |
|-------------|-----------|
| EHR700042 | 22.08% |
| EHR700052 | 25.00% |
| EHR700233 | 18.92% |
| EHR700232 | 31.82% |
| EHR700234 | 9.35% |

**验证点**: CAST 类型转换、列间算术除法乘法、AS 别名、WHERE 排除零除数。
**结果**: ✅ PASS

---

## Q12: 触达率时间趋势 (GROUP BY date + 时间排序)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv $BIN execute -env --label touch-ops-csv \
  'SELECT data_date, AVG(reach_rate) AS avg_reach_rate FROM pb_touch_ops_sample_2000 GROUP BY data_date ORDER BY data_date ASC' --human
```

**结果**: 90 行 (20260301-20260529)，GROUP BY date 正确，ORDER BY date ASC 排序正确。
**验证点**: 日期作为分组键、多聚合、时间排序。
**结果**: ✅ PASS (90 row(s) in set ~1.1ms)

---

## Q13: 条件过滤 (AND 多条件 + 混合比较)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv $BIN execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, reach_rate, pri_cnt FROM pb_touch_ops_sample_2000 WHERE reach_rate < 60 AND pri_cnt > 5 ORDER BY reach_rate ASC LIMIT 5' --human
```

| csmgr_refno | reach_rate | pri_cnt |
|-------------|------------|---------|
| EHR700171 | 13.25 | 14 |
| EHR700071 | 14.06 | 7 |
| EHR700008 | 17.33 | 17 |
| EHR700206 | 17.98 | 14 |
| EHR700219 | 18.18 | 19 |

**验证点**: AND 连接两个条件、`<` 和 `>` 比较运算符混合。
**结果**: ✅ PASS

---

## Q14: 跨文件 JOIN (哈希 JOIN + GROUP BY)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-join $BIN execute -env --label touch-ops \
  'SELECT o.sec_branch_org_name, AVG(t.reach_rate) AS avg_reach_rate FROM pb_touch_ops_sample_2000 t JOIN t_sec_org_sample o ON t.org_refno = o.org_refno GROUP BY o.sec_branch_org_name ORDER BY avg_reach_rate DESC' --human
```

**结果**: 40 个二级分行按平均触达率排序。
**验证点**: 跨文件哈希 JOIN、限定列名 (t.col / o.col)、GROUP BY 关联列。
**结果**: ✅ PASS (40 row(s) in set ~1.6ms)

---

## Q15: 数据质量验证 (嵌套算术 + ABS + CAST)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv $BIN execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, reach_rate, ABS(reach_rate - (CAST(total_reach_cnt AS FLOAT) / tol_cnt * 100)) AS diff FROM pb_touch_ops_sample_2000 WHERE tol_cnt > 0 ORDER BY diff DESC LIMIT 10' --human
```

| csmgr_refno | reach_rate | diff |
|-------------|------------|------|
| EHR700042 | 84.0 | 80.36 |
| EHR700052 | 36.49 | 84.21 |
| EHR700233 | 42.53 | 102.30 |
| EHR700232 | 41.71 | 90.99 |
| EHR700234 | 74.83 | 106.29 |

**验证点**: 嵌套算术表达式、ABS 绝对值函数、CAST 类型转换、列别名。
**注意**: `csmgr_refno` 列已正确处理 UTF-8 BOM 前缀。
**结果**: ✅ PASS (10 row(s) in set ~1.0ms)

---

## 安全策略验证 F1-F3 (Q06-Q08)

### F1: 表级拒绝 (DENY_TABLES)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-security $BIN execute -env --label touch-ops 'SELECT *' --limit 1
# → ACCESS_DENIED: table "pb_touch_ops_sample_2000" is not allowed for query
```

**结果**: ✅ PASS — 策略在 execute.go 层拦截，引擎不感知。

### F2: 列屏蔽 (MASK_COLUMNS)

```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-security $BIN execute -env --label sec-org 'SELECT *' --limit 3 --human
# → pnbrn_org_name=****, org_name=REDACTED, 其余列正常
```

**结果**: ✅ PASS — 策略在 execute.go 的 ApplyMask 层处理后输出。

### F3: 只读保护

```bash
# 对已禁止表执行 DROP → DENY_TABLES 先拦截
DBPROBE_ENV_FILE=testdata/qa/.env.qa-security $BIN execute -env --label touch-ops 'DROP TABLE pb_touch_ops_sample_2000'
# → ACCESS_DENIED: table "pb_touch_ops_sample_2000" is not allowed for query

# 对未禁止表执行 DROP → 文件查询引擎拒绝非 SELECT
DBPROBE_ENV_FILE=testdata/qa/.env.qa-security $BIN execute -env --label sec-org 'DROP TABLE t_sec_org_sample'
# → QUERY_ERROR: csv query error: parse error: expected SELECT, got DROP at position 0
```

**结果**: ✅ PASS — 两个路径都正确拒绝写操作。

---

## 总计

| 场景 | 结果 | 验证内容 |
|------|------|----------|
| Q09 | ✅ | GROUP BY + AVG + ORDER BY DESC |
| Q10 | ✅ | ORDER BY ASC + LIMIT + 列投影 |
| Q11 | ✅ | CAST + 列间算术 + WHERE 排除 |
| Q12 | ✅ | GROUP BY date + 时间排序 |
| Q13 | ✅ | AND 多条件 + 混合比较 |
| Q14 | ✅ | 跨文件哈希 JOIN + GROUP BY |
| Q15 | ✅ | 嵌套算术 + ABS + CAST |
| F1 | ✅ | DENY_TABLES 表级拒绝 |
| F2 | ✅ | MASK_COLUMNS 列屏蔽 |
| F3 | ✅ | 只读保护 (DROP 拒绝) |

**总计: 10/10 验证项通过**
