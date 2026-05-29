# Q14: 组织+触达 JOIN（跨文件 JOIN + GROUP BY + ORDER BY）

## 问题
各二级分行（sec_branch_org_name）的平均客户触达率是多少？请使用触达运营表和机构表关联查询。

## 测试命令
```bash
DBPROBE_ENV_FILE=testdata/qa/.env.qa-touch-join dbexplain execute -env --label touch-ops \
  'SELECT o.sec_branch_org_name, AVG(t.reach_rate) AS avg_reach_rate FROM pb_touch_ops_sample_2000 t JOIN t_sec_org_sample o ON t.org_refno = o.org_refno GROUP BY o.sec_branch_org_name ORDER BY avg_reach_rate DESC' --human
```

## 预期输出

| 检查点 | 预期值 |
|---------|--------|
| 行数 | 多个二级分行 |
| 列数 | 2（sec_branch_org_name, avg_reach_rate） |
| 关联 | 通过 org_refno 正确匹配 |
| 排序 | 按 avg_reach_rate DESC |

## 验证点
- [ ] 跨文件 JOIN 正确（触达表 × 机构表）
- [ ] 限定列名 o.xxx 和 t.xxx 正确解析
- [ ] GROUP BY 在 JOIN 结果上正确聚合
- [ ] AVG(reach_rate) 计算正确
- [ ] ORDER BY 聚合结果正确

## 涉及的文件查询引擎功能
- 跨文件哈希 JOIN
- 限定列名（表别名.列名）
- GROUP BY + AVG 聚合
- ORDER BY DESC
