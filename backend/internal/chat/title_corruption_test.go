package chat

import "testing"

func TestIsCorruptedChatTitle(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{"", true},
		{"??????? orders ? SKU-TEST-002?", true},
		{"Сделай ABC-анализ товарной базы", false},
		{"Разберите 8 SKU без заказов", false},
		{"Новый чат", false},
	}
	for _, tc := range tests {
		if got := IsCorruptedChatTitle(tc.title); got != tc.want {
			t.Fatalf("IsCorruptedChatTitle(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}
