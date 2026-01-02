package app_selector

import (
	"testing"
)

func TestOperator_String(t *testing.T) {
	tests := []struct {
		name     string
		operator Operator
		want     string
	}{
		{
			name:     "Equal operator",
			operator: Eq,
			want:     "=",
		},
		{
			name:     "Not Equal operator",
			operator: Ne,
			want:     "!=",
		},
		{
			name:     "Unknown operator",
			operator: 999, // Invalid operator
			want:     "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.operator.String(); got != tt.want {
				t.Errorf("Operator.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelector_String(t *testing.T) {
	tests := []struct {
		name     string
		selector *Selector
		want     string
	}{
		{
			name: "Equal operator selector",
			selector: &Selector{
				Key:      "app",
				Value:    "myapp",
				Operator: Eq,
			},
			want: "app=myapp",
		},
		{
			name: "Not Equal operator selector",
			selector: &Selector{
				Key:      "env",
				Value:    "prod",
				Operator: Ne,
			},
			want: "env!=prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.selector.String(); got != tt.want {
				t.Errorf("Selector.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Selector
		wantErr bool
	}{
		{
			name:  "Valid equal selector with =",
			input: "app=myapp",
			want: &Selector{
				Key:      "app",
				Value:    "myapp",
				Operator: Eq,
			},
			wantErr: false,
		},
		{
			name:  "Valid equal selector with ==",
			input: "app==myapp",
			want: &Selector{
				Key:      "app",
				Value:    "myapp",
				Operator: Eq,
			},
			wantErr: false,
		},
		{
			name:  "Valid not equal selector",
			input: "env!=prod",
			want: &Selector{
				Key:      "env",
				Value:    "prod",
				Operator: Ne,
			},
			wantErr: false,
		},
		{
			name:    "Invalid format - no operator",
			input:   "invalid",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid format - empty key",
			input:   "=value",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid format - empty value",
			input:   "key=",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid format - key contains =",
			input:   "key=value=extra",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid format - key contains !",
			input:   "my!key=value",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid format - value contains =",
			input:   "key=value=extra",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid format - value contains !",
			input:   "key=value!",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Key != tt.want.Key || got.Value != tt.want.Value || got.Operator != tt.want.Operator {
				t.Errorf("FromString() = %v, want %v", got, tt.want)
			}
		})
	}
}
