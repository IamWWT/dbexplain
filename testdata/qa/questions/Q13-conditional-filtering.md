# Q13: 条件过滤阈值（AND 多条件 + 混合比较运算符）

## 问题
哪些客户经理的触达率（reach_rate）低于 60% 但私人银行客户数（pri_cnt）超过 5 人？

## 测试命令
```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv dbexplain execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, reach_rate, pri_cnt FROM pb_touch_ops_sample_2000 WHERE reach_rate < 60 AND pri_cnt > 5 ORDER BY reach_rate ASC' --human
```

## 预期输出

| 检查点 | 预期值 |
|---------|--------|
| 行数 | 符合条件的数据行 |
| 列数 | 3（csmgr_refno, reach_rate, pri_cnt） |
| reach_rate | 全部 < 60 |
| pri_cnt | 全部 > 5 |

## 验证点
- [ ] WHERE reach_rate < 60 正确过滤
- [ ] AND pri_cnt > 5 多条件组合
- [ ] ORDER BY reach_rate ASC 排序正确
- [ ] 两个条件同时满足的行才返回

## 涉及的文件查询引擎功能
- WHERE 多条件 AND
- 比较运算符 < 和 >
- ORDER BY ASC
- 列投影
