# Q10: 触达率最低的客户经理（ORDER BY ASC + LIMIT + 列投影）

## 问题
客户触达率（reach_rate）最低的 10 位客户经理（csmgr_refno）是谁？他们分别属于哪个省份（pnbrn_org_name）？

## 测试命令
```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv dbexplain execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, pnbrn_org_name, reach_rate, tol_cnt FROM pb_touch_ops_sample_2000 ORDER BY reach_rate ASC LIMIT 10' --human
```

## 预期输出

| 检查点 | 预期值 |
|---------|--------|
| 行数 | 10（LIMIT 10） |
| 列数 | 4（csmgr_refno, pnbrn_org_name, reach_rate, tol_cnt） |
| 排序 | 按 reach_rate ASC，最低在前 |
| reach_rate | 最低的触达率应接近 0 |

## 验证点
- [ ] ORDER BY reach_rate ASC 正确返回最低的 10 条
- [ ] LIMIT 10 正确截断
- [ ] 列投影正确（只选择 4 列，而非 SELECT *）
- [ ] 含 tol_cnt（名下总客户数）有助于理解触达率低的背景

## 涉及的文件查询引擎功能
- ORDER BY ASC
- LIMIT
- 列投影（非 SELECT *）
