package apply

import "testing"

func TestAggregateStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		states []string
		want   string
	}{
		{name: "pending", states: []string{"pending", "pending"}, want: "pending"},
		{name: "running", states: []string{"assigned", "completed"}, want: "running"},
		{name: "completed", states: []string{"completed", "completed"}, want: "completed"},
		{name: "failed", states: []string{"completed", "failed"}, want: "failed"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := AggregateStatus(tt.states); got != tt.want {
				t.Fatalf("AggregateStatus(%v) = %q, want %q", tt.states, got, tt.want)
			}
		})
	}
}
