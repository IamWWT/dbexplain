# Q11: 企微渠道效能（CAST + 列间算术 + WHERE + ORDER BY DESC）

## 问题
哪些客户经理的企微互动占比最高？企微互动客户数（wecom_interact_cnt）占总互动客户数（total_interact_cnt）的百分比是多少？

## 测试命令
```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-csv dbexplain execute -env --label touch-ops-csv \
  'SELECT csmgr_refno, CAST(wecom_interact_cnt AS FLOAT) / total_interact_cnt * 100 AS wecom_pct FROM pb_touch_ops_sample_2000 WHERE total_interact_cnt > 0 ORDER BY wecom_pct DESC LIMIT 10' --human
```

## 预期输出

| 检查点 | 预期值 |
|---------|--------|
| 行数 | 10（LIMIT 10） |
| 列数 | 2（csmgr_refno, wecom_pct） |
| wecom_pct | 浮点数，范围 0~100 |
| 排序 | 按 wecom_pct DESC，最高在前 |

## 验证点
- [ ] CAST(wecom_interact_cnt AS FLOAT) 正确转换类型
- [ ] 列间算术（除法 + 乘法）正确
- [ ] WHERE total_interact_cnt > 0 排除除零风险
- [ ] AS wecom_pct 别名正确显示
- [ ] ORDER BY 对别名列正确排序

## 涉及的文件查询引擎功能
- CAST() 类型转换
- 列间算术表达式
- WHERE 条件过滤
- ORDER BY DESC
- AS 列别名
