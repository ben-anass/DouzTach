package notifications

import (
	"errors"
	"testing"
)

func TestValidatePage(t *testing.T) {
	tests := []struct {
		name    string
		size    int32
		offset  int32
		wantErr bool
	}{
		{name: "minimum", size: 1},
		{name: "maximum", size: 100},
		{name: "zero size", size: 0, wantErr: true},
		{name: "oversized", size: 101, wantErr: true},
		{name: "negative offset", size: 1, offset: -1, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePage(test.size, test.offset)
			if errors.Is(err, ErrInvalidInput) != test.wantErr {
				t.Fatalf("validatePage() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
