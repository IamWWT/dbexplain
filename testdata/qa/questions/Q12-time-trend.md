# Q12: 触达率时间趋势（GROUP BY date + 聚合 + 时间排序）

## 问题
不同日期的平均客户触达率（reach_rate）分别是多少？触达率随时间的变化趋势是怎样的？

## 测试命令
```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv dbexplain execute -env --label touch-ops-csv \
  'SELECT data_date, AVG(reach_rate) AS avg_reach_rate FROM pb_touch_ops_sample_2000 GROUP BY data_date ORDER BY data_date ASC' --human
```

## 预期输出

| 检查点 | 预期值 |
|---------|--------|
| 行数 | 多个日期（大约数十个交易日） |
| 列数 | 2（data_date, avg_reach_rate） |
| data_date | 按日期 ASC 排序，如 20260303, 20260304 ... |
| avg_reach_rate | 浮点数，每日平均值 |

## 验证点
- [ ] GROUP BY data_date 正确按日期分组
- [ ] AVG(reach_rate) 每日平均正确
- [ ] ORDER BY data_date ASC 正确时间排序
- [ ] 日期字段 data_date 作为整数处理（排序正确）

## 涉及的文件查询引擎功能
- GROUP BY 日期字段
- AVG() 聚合函数
- ORDER BY ASC
