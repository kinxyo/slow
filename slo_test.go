package main

import (
	"strings"
	"testing"
)

func TestSloFlags(t *testing.T) {
	tt := []struct {
		args  []string
		check func() bool
	}{
		{
			args: []string{"-p"},
			check: func() bool {
				return pager
			},
		},
		{
			args: []string{"-s", "light"},
			check: func() bool {
				return style == "light"
			},
		},
		{
			args: []string{"-w", "40"},
			check: func() bool {
				return width == 40
			},
		},
	}

	for _, v := range tt {
		err := rootCmd.ParseFlags(v.args)
		if err != nil {
			t.Fatal(err)
		}
		if !v.check() {
			t.Errorf("Parsing flag failed: %s", v.args)
		}
	}
}

func TestDefaultConfigDoesNotPinTheWidth(t *testing.T) {
	if strings.Contains(strings.ReplaceAll(defaultConfig, "# width", ""), "width:") {
		t.Error("a width in the default config file overrides the column for everyone")
	}
}
