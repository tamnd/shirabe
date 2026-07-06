package execsrc

import (
	"strconv"
	"strings"
)

// get walks a dot path through decoded JSON: map keys and array indices.
// "items.0.title" reads doc["items"][0]["title"]. Returns nil when any hop
// is missing, so manifests degrade to empty fields instead of errors.
func get(doc any, path string) any {
	cur := doc
	for seg := range strings.SplitSeq(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[seg]
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(node) {
				return nil
			}
			cur = node[i]
		default:
			return nil
		}
		if cur == nil {
			return nil
		}
	}
	return cur
}

// asString renders an extracted value for an envelope field.
func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	default:
		return ""
	}
}

// asFloat renders an extracted value for a numeric field.
func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(strings.ReplaceAll(t, ",", ""), 64)
		return f, err == nil
	default:
		return 0, false
	}
}
