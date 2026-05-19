package schema

import "testing"

func TestInferComment(t *testing.T) {
	tests := []struct {
		name, typ, sample string
		want              string
	}{
		// id pattern
		{name: "id", typ: "int", sample: "1", want: "标识符"},
		{name: "user_id", typ: "int", sample: "42", want: "标识符"},
		{name: "product_id", typ: "varchar", sample: "P001", want: "标识符"},
		{name: "order_item_id", typ: "int", sample: "100", want: "标识符"},

		// name/title pattern
		{name: "name", typ: "varchar", sample: "Alice", want: "名称"},
		{name: "full_name", typ: "varchar", sample: "Bob Smith", want: "名称"},
		{name: "title", typ: "varchar", sample: "Hello", want: "名称"},

		// time/date pattern — column must contain "time" or "date" literally
		{name: "created_time", typ: "timestamp", sample: "2024-01-01", want: "时间"},
		{name: "updated_time", typ: "datetime", sample: "2024-06-15", want: "时间"},
		{name: "order_date", typ: "date", sample: "2024-01-01", want: "时间"},

		// amount/price/total pattern
		{name: "amount", typ: "decimal", sample: "99.99", want: "金额/数量"},
		{name: "price", typ: "decimal", sample: "49.99", want: "金额/数量"},
		{name: "total", typ: "int", sample: "1000", want: "金额/数量"},
		{name: "total_amount", typ: "decimal", sample: "500.00", want: "金额/数量"},

		// status/state pattern
		{name: "status", typ: "varchar", sample: "active", want: "状态"},
		{name: "order_state", typ: "varchar", sample: "pending", want: "状态"},

		// flag/is_ pattern
		{name: "is_active", typ: "bool", sample: "true", want: "标志位"},
		{name: "is_deleted", typ: "bool", sample: "false", want: "标志位"},
		{name: "flag", typ: "int", sample: "1", want: "标志位"},

		// email
		{name: "email", typ: "varchar", sample: "a@b.com", want: "电子邮箱"},
		{name: "user_email", typ: "varchar", sample: "user@test.com", want: "电子邮箱"},

		// phone/mobile
		{name: "phone", typ: "varchar", sample: "123456", want: "电话号码"},
		{name: "mobile", typ: "varchar", sample: "987654", want: "电话号码"},
		{name: "phone_number", typ: "varchar", sample: "555-1234", want: "电话号码"},

		// ip
		{name: "ip", typ: "varchar", sample: "1.2.3.4", want: "IP 地址"},
		{name: "ip_address", typ: "varchar", sample: "10.0.0.1", want: "IP 地址"},
		{name: "client_ip", typ: "varchar", sample: "192.168.1.1", want: "IP 地址"},

		// url
		{name: "url", typ: "text", sample: "http://x", want: "URL"},
		{name: "redirect_url", typ: "varchar", sample: "https://example.com", want: "URL"},

		// image/avatar — must NOT also contain "url" (url check has higher priority)
		{name: "image", typ: "varchar", sample: "img.png", want: "图片 URL"},
		{name: "avatar", typ: "varchar", sample: "avatar.jpg", want: "图片 URL"},
		// img_url matches "url" (case 9) before "img" (case 10) due to switch order
		{name: "img_url", typ: "text", sample: "pic.png", want: "URL"},

		// description/desc
		{name: "description", typ: "text", sample: "desc", want: "描述"},
		{name: "desc", typ: "varchar", sample: "summary", want: "描述"},
		{name: "product_desc", typ: "text", sample: "A product", want: "描述"},

		// key
		{name: "api_key", typ: "varchar", sample: "abc123", want: "键"},
		{name: "ssh_key", typ: "text", sample: "ssh-rsa...", want: "键"},

		// type
		{name: "type", typ: "varchar", sample: "X", want: "类型"},
		{name: "order_type", typ: "varchar", sample: "online", want: "类型"},

		// json
		{name: "data", typ: "json", sample: "...", want: "JSON 数据"},
		{name: "payload", typ: "jsonb", sample: "{}", want: "JSON 数据"},
		{name: "config", typ: "json", sample: `{"key":"val"}`, want: "JSON 数据"},

		// Sample value fallback
		{name: "unknown_col", typ: "varchar", sample: "short", want: "示例: short"},
		{name: "unknown_col", typ: "text", sample: "this is a very long sample value for testing", want: "示例: this is a very long …"},
		{name: "unknown_col", typ: "varchar", sample: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/"+tt.sample, func(t *testing.T) {
			got := InferComment(tt.name, tt.typ, tt.sample)
			if got != tt.want {
				t.Errorf("InferComment(%q, %q, %q) = %q, want %q",
					tt.name, tt.typ, tt.sample, got, tt.want)
			}
		})
	}
}

func TestInferComment_Ordering(t *testing.T) {
	// id-containing column with another pattern: "id" matches before "name"
	// Even though "channel_id" contains "id" and has "id" suffix, but also
	// contains "name"? No, it doesn't. But "identifier" would still match "id" first.
	got := InferComment("identifier", "varchar", "X")
	// Contains "id" AND HasSuffix "id"? "identifier" has suffix "ier", not "id"
	// So it should NOT match the "id" rule.
	// Contains "name"? No.
	// Contains "time"? No.
	// Falls through to sample fallback:
	if got != "示例: X" {
		t.Errorf("InferComment(identifier, varchar, X) = %q, want %q", got, "示例: X")
	}
}
