# Q09: 跨省触达率排行（GROUP BY + AVG + ORDER BY DESC）

## 问题
各省份（pnbrn_org_name）的平均客户触达率（reach_rate）是多少？请按触达率从高到低排序。

## 测试命令
```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv dbexplain execute -env --label touch-ops-csv \
  'SELECT pnbrn_org_name, AVG(reach_rate) AS avg_reach_rate FROM pb_touch_ops_sample_2000 GROUP BY pnbrn_org_name ORDER BY avg_reach_rate DESC' --human
```

## 预期输出

| 检查点 | 预期值 |
|---------|--------|
| 行数 | ~8（8个省份） |
| 列数 | 2（pnbrn_org_name, avg_reach_rate） |
| 排序 | 按 avg_reach_rate DESC，最高在前 |
| 广东分行 | 应在 Top 3 |

## 验证点
- [ ] GROUP BY pnbrn_org_name 正确聚合（8个省份）
- [ ] AVG(reach_rate) 计算正确
- [ ] ORDER BY avg_reach_rate DESC 正确排序
- [ ] 结果不含非聚合列
- [ ] AS 别名（avg_reach_rate）在输出中正确显示

## 涉及的文件查询引擎功能
- GROUP BY 聚合
- AVG() 聚合函数
- ORDER BY DESC
- AS 列别名
