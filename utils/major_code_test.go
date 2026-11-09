package utils

import "testing"

func TestMajorCodeBase(t *testing.T) {
	for name, want := range map[string]string{
		"Quản trị kinh doanh": "QTKD", "Đồ họa đa phương tiện": "DHDPT",
		"  Công nghệ - thông tin  ": "CNTT", "Điện–Điện tử": "DDT", "Marketing": "M", "123": "CN",
	} {
		if got := MajorCodeBase(name); got != want {
			t.Errorf("%q: want %s got %s", name, want, got)
		}
	}
}
