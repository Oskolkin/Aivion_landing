package ozon

import (
	"encoding/json"
	"testing"
)

func TestPickPrimaryImageURL(t *testing.T) {
	tests := []struct {
		name    string
		primary FlexibleStringList
		images  FlexibleStringList
		want    string
	}{
		{
			name:    "primary array first element",
			primary: FlexibleStringList{"https://cdn/1.jpg"},
			images:  FlexibleStringList{"https://cdn/2.jpg"},
			want:    "https://cdn/1.jpg",
		},
		{
			name:    "fallback to images array",
			primary: FlexibleStringList{},
			images:  FlexibleStringList{"https://cdn/2.jpg"},
			want:    "https://cdn/2.jpg",
		},
		{
			name:    "both empty arrays",
			primary: FlexibleStringList{},
			images:  FlexibleStringList{},
			want:    "",
		},
		{
			name:    "skip empty strings in primary array",
			primary: FlexibleStringList{"", "https://cdn/ok.jpg"},
			images:  FlexibleStringList{"https://cdn/fallback.jpg"},
			want:    "https://cdn/ok.jpg",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := PickPrimaryImageURL(tc.primary, tc.images); got != tc.want {
				t.Fatalf("PickPrimaryImageURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestProductInfoListV3_primaryImageAndImagesArrays is the primary contract test for
// POST /v3/product/info/list: both fields are JSON arrays of URL strings.
func TestProductInfoListV3_primaryImageAndImagesArrays(t *testing.T) {
	const raw = `{
		"items": [
			{
				"id": 9001,
				"offer_id": "OFFER-1",
				"sku": 555001,
				"name": "Товар из v3",
				"price": "1990.00",
				"old_price": "2490.00",
				"min_price": "1790.00",
				"primary_image": [
					"https://cdn.ozon.ru/primary-a.jpg",
					"https://cdn.ozon.ru/primary-b.jpg"
				],
				"images": [
					"https://cdn.ozon.ru/img-1.jpg",
					"https://cdn.ozon.ru/img-2.jpg"
				]
			}
		]
	}`

	var parsed ProductInfoListResult
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("unmarshal v3 product info list: %v", err)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("items len=%d", len(parsed.Items))
	}

	item := parsed.Items[0]
	if len(item.PrimaryImage) != 2 || item.PrimaryImage[0] != "https://cdn.ozon.ru/primary-a.jpg" {
		t.Fatalf("primary_image=%v", item.PrimaryImage)
	}
	if len(item.Images) != 2 || item.Images[1] != "https://cdn.ozon.ru/img-2.jpg" {
		t.Fatalf("images=%v", item.Images)
	}

	got := PickPrimaryImageURL(item.PrimaryImage, item.Images)
	if got != "https://cdn.ozon.ru/primary-a.jpg" {
		t.Fatalf("primary url = %q", got)
	}

	stored := PickPrimaryImageURL(item.PrimaryImage, item.Images)
	imageJSON := ImageURLsJSON(item.Images)
	if stored == "" {
		t.Fatal("expected stored primary url")
	}
	if imageJSON == nil {
		t.Fatal("expected image_urls json")
	}
}

func TestProductInfoListV3_emptyImageArrays(t *testing.T) {
	const raw = `{
		"items": [{
			"id": 2,
			"sku": 2,
			"name": "No images",
			"primary_image": [],
			"images": []
		}]
	}`

	var parsed ProductInfoListResult
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	item := parsed.Items[0]
	if got := PickPrimaryImageURL(item.PrimaryImage, item.Images); got != "" {
		t.Fatalf("expected empty primary, got %q", got)
	}
}

func TestFlexibleStringListUnmarshal_v3ArrayFormat(t *testing.T) {
	t.Run("v3 array of urls", func(t *testing.T) {
		var list FlexibleStringList
		raw := `["https://cdn.ozon.ru/a.jpg","https://cdn.ozon.ru/b.jpg"]`
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 || list[0] != "https://cdn.ozon.ru/a.jpg" {
			t.Fatalf("list=%v", list)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		var list FlexibleStringList
		if err := json.Unmarshal([]byte(`[]`), &list); err != nil {
			t.Fatal(err)
		}
		if len(list) != 0 {
			t.Fatalf("list=%v", list)
		}
	})
}

func TestFlexibleStringListUnmarshal_legacySingleString(t *testing.T) {
	var list FlexibleStringList
	if err := json.Unmarshal([]byte(`"https://cdn/x.jpg"`), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0] != "https://cdn/x.jpg" {
		t.Fatalf("list=%v", list)
	}
}
