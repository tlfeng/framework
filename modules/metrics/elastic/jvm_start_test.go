package elastic

import "testing"

func TestEstimateJVMStart(t *testing.T) {
	for _, tc := range []struct {
		name string
		jvm  map[string]interface{}
		want int64
	}{
		{"valid", map[string]interface{}{"timestamp": 10000.0, "uptime_in_millis": 1000.0}, 9000},
		{"fresh", map[string]interface{}{"timestamp": 10000.0, "uptime_in_millis": 0.0}, 10000},
		{"missing", map[string]interface{}{"timestamp": 10000.0}, 0},
		{"negative", map[string]interface{}{"timestamp": 10000.0, "uptime_in_millis": -1.0}, 0},
		{"fractional", map[string]interface{}{"timestamp": 10000.0, "uptime_in_millis": 1.5}, 0},
		{"invalid start", map[string]interface{}{"timestamp": 1000.0, "uptime_in_millis": 1001.0}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := estimateJVMStart(tc.jvm)
			if got != tc.want || ok != (tc.want != 0) {
				t.Fatalf("got %d, %v; want %d", got, ok, tc.want)
			}
		})
	}
}
