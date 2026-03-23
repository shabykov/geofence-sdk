package pkg

import (
	"encoding/json"
	"strings"

	"github.com/tidwall/geojson"
)

// Props provides typed access to feature properties.
type Props map[string]any

func (p Props) Str(key string) string {
	if v, ok := p[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (p Props) Int(key string) int {
	if v, ok := p[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case json.Number:
			i, _ := n.Int64()
			return int(i)
		}
	}
	return 0
}

func (p Props) Float(key string) float64 {
	if v, ok := p[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		}
	}
	return 0
}

func (p Props) Has(key string) bool {
	_, ok := p[key]
	return ok
}

func parseProps(feat *geojson.Feature) Props {
	raw := feat.Members()
	if raw == "" {
		return Props{}
	}

	var outer map[string]any
	if err := json.Unmarshal([]byte(raw), &outer); err != nil {
		return Props{}
	}

	if propsRaw, ok := outer["properties"]; ok {
		if pm, ok := propsRaw.(map[string]any); ok {
			return Props(pm)
		}
	}

	props := Props{}
	for k, v := range outer {
		lower := strings.ToLower(k)
		if lower == "type" || lower == "geometry" || lower == "bbox" {
			continue
		}
		props[k] = v
	}
	return props
}
