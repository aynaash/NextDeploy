package nextcore

import "testing"

func TestParseConfigObjectProjectsImageQualitiesAndRemoteSearch(t *testing.T) {
	config, err := parseConfigObject(map[string]interface{}{
		"images": map[string]interface{}{
			"qualities": []interface{}{50, 75},
			"remotePatterns": []interface{}{
				map[string]interface{}{
					"protocol": "https",
					"hostname": "images.example.com",
					"pathname": "/assets/**",
					"search":   "?v=1",
				},
				map[string]interface{}{
					"hostname": "cdn.example.com",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if config.Images == nil || len(config.Images.Qualities) != 2 ||
		config.Images.Qualities[0] != 50 || config.Images.Qualities[1] != 75 {
		t.Fatalf("image qualities = %#v", config.Images)
	}
	patterns := config.Images.RemotePatterns
	if len(patterns) != 2 {
		t.Fatalf("remote patterns = %#v", patterns)
	}
	if !patterns[0].SearchConfigured || patterns[0].Search != "?v=1" {
		t.Fatalf("configured search = %q, configured=%v", patterns[0].Search, patterns[0].SearchConfigured)
	}
	if patterns[1].SearchConfigured {
		t.Fatal("omitted remote pattern search was treated as explicitly configured")
	}
}
