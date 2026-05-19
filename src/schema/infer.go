package schema

import "strings"

// InferComment 基于列名、类型和样本值推断注释
func InferComment(colName, colType, sampleValue string) string {
	name := strings.ToLower(colName)
	switch {
	case strings.Contains(name, "id") && strings.HasSuffix(name, "id"):
		return "标识符"
	case strings.Contains(name, "name") || strings.Contains(name, "title"):
		return "名称"
	case strings.Contains(name, "time") || strings.Contains(name, "date"):
		return "时间"
	case strings.Contains(name, "amount") || strings.Contains(name, "price") || strings.Contains(name, "total"):
		return "金额/数量"
	case strings.Contains(name, "status") || strings.Contains(name, "state"):
		return "状态"
	case strings.Contains(name, "flag") || strings.Contains(name, "is_"):
		return "标志位"
	case strings.Contains(name, "email"):
		return "电子邮箱"
	case strings.Contains(name, "phone") || strings.Contains(name, "mobile"):
		return "电话号码"
	case strings.HasPrefix(name, "ip_") || strings.HasSuffix(name, "_ip") || strings.Contains(name, "_ip_") || name == "ip":
		return "IP 地址"
	case strings.Contains(name, "url"):
		return "URL"
	case strings.Contains(name, "img") || strings.Contains(name, "image") || strings.Contains(name, "avatar"):
		return "图片 URL"
	case strings.Contains(name, "description") || strings.Contains(name, "desc"):
		return "描述"
	case strings.Contains(name, "key"):
		return "键"
	case strings.Contains(name, "type"):
		return "类型"
	case strings.Contains(name, "json") || strings.Contains(colType, "json"):
		return "JSON 数据"
	}
	if sampleValue != "" {
		if len(sampleValue) > 20 {
			return "示例: " + sampleValue[:20] + "…"
		}
		return "示例: " + sampleValue
	}
	return ""
}