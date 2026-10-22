package k8s

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func str(obj map[string]any, path ...string) string {
	v, _, _ := unstructured.NestedString(obj, path...)
	return v
}

func i64(obj map[string]any, path ...string) int64 {
	v, _, _ := unstructured.NestedInt64(obj, path...)
	return v
}

func i32(obj map[string]any, path ...string) int32 { return int32(i64(obj, path...)) }

func flag(obj map[string]any, path ...string) bool {
	v, _, _ := unstructured.NestedBool(obj, path...)
	return v
}

func stringMap(obj map[string]any, path ...string) map[string]string {
	v, _, _ := unstructured.NestedStringMap(obj, path...)
	return v
}

func items(obj map[string]any, path ...string) []map[string]any {
	raw, _, _ := unstructured.NestedSlice(obj, path...)
	out := make([]map[string]any, 0, len(raw))
	for _, it := range raw {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func selector(obj map[string]any, path ...string) string {
	raw, found, _ := unstructured.NestedMap(obj, path...)
	if !found {
		return ""
	}
	var sel metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &sel); err != nil {
		return ""
	}
	return metav1.FormatLabelSelector(&sel)
}

func timestamp(obj map[string]any, path ...string) *time.Time {
	raw := str(obj, path...)
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}
