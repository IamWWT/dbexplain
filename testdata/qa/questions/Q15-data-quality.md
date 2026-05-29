# Q15: 数据质量验证（嵌套算术 + ABS + CAST + 多条件 WHERE）

## 问题
哪些客户经理的触达率（reach_rate）与计算触达率（total_reach_cnt / tol_cnt × 100）之间存在超过 5 个百分点的偏差？

## 测试命令
```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv dbexplain execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, reach_rate, ABS(reach_rate - (CAST(total_reach_cnt AS FLOAT) / tol_cnt * 100)) AS diff FROM pb_touch_ops_sample_2000 WHERE tol_cnt > 0 ORDER BY diff DESC LIMIT 10' --human
```

## 预期输出

| 检查点 | 预期值 |
|--------|--------|
| 行数 | 10（LIMIT 10） |
| 列数 | 3（csmgr_refno, reach_rate, diff） |
| diff | 非负浮点数 |
| 排序 | 按 diff DESC，偏差最大的在前 |

## 验证点
- [ ] CAST(total_reach_cnt AS FLOAT) 正确转换
- [ ] 嵌套算术表达式（除法 × 100）正确
- [ ] ABS() 确保非负偏差
- [ ] WHERE tol_cnt > 0 排除除零
- [ ] 多步计算逻辑正确（验证 reach_rate 与计算值是否一致）

## 涉及的文件查询引擎功能
- CAST() 类型转换
- 嵌套算术表达式
- ABS() 数学函数
- WHERE 过滤
- ORDER BY DESC + LIMIT
