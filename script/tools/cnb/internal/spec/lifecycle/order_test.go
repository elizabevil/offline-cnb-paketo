package lifecycle

import "testing"

const orderSample = `
[[order]]
  [[order.group]]
  id = "paketo-buildpacks/go"

  [[order.group]]
  id = "paketo-buildpacks/go-build"

[[order]]
  [[order.group]]
  id = "paketo-buildpacks/nodejs"
`

func TestParseOrder(t *testing.T) {
	doc, warns, err := ParseOrder([]byte(orderSample))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 0 {
		t.Fatalf("不应有告警: %v", warns)
	}
	if len(doc.Order) != 2 || len(doc.Order[0].Group) != 2 {
		t.Fatalf("解析结果不符: %+v", doc)
	}
}

func TestParseOrderUnknownKey(t *testing.T) {
	_, warns, err := ParseOrder([]byte("foo = 1\n[[order]]\n  [[order.group]]\n  id = \"x\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warns) != 1 || warns[0].Path != "foo" {
		t.Fatalf("应报告未知键 foo: %+v", warns)
	}
}

func TestFilterOrder(t *testing.T) {
	doc, _, _ := ParseOrder([]byte(orderSample))
	kept := doc.Filter("buildpack go 0.0.0", map[string]string{"go": "paketo-buildpacks/go"})
	if len(kept.Order) != 1 || kept.Order[0].Group[0].ID != "paketo-buildpacks/go" {
		t.Fatalf("过滤结果不符: %+v", kept)
	}
	if all := doc.Filter("", nil); len(all.Order) != 2 {
		t.Fatal("空 required 应原样返回")
	}
}

func TestFilterWithAlias(t *testing.T) {
	doc, _, _ := ParseOrder([]byte(orderSample))
	kept := doc.Filter("buildpack bk-buildpack-go 1.0", map[string]string{"bk-buildpack-go": "paketo-buildpacks/go"})
	if len(kept.Order) != 1 {
		t.Fatalf("别名未命中: %+v", kept)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	doc, _, _ := ParseOrder([]byte(orderSample))
	data, err := doc.Encode()
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := ParseOrder(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Order) != len(doc.Order) {
		t.Fatalf("round-trip 丢块: %d != %d", len(again.Order), len(doc.Order))
	}
}
