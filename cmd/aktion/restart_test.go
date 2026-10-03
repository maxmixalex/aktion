package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestRestartBuildsBeforeStopping(t *testing.T) {
	for _, tc := range []struct {
		name, failure string
		want          []string
		code          int
	}{
		{"success", "", []string{"features", "build", "down", "up -d --no-build"}, 0},
		{"feature build failure", "features", []string{"features"}, 1},
		{"listener build failure", "build", []string{"features", "build"}, 7},
		{"stop failure", "down", []string{"features", "build", "down"}, 7},
		{"start failure", "up -d --no-build", []string{"features", "build", "down", "up -d --no-build"}, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			code := restartStack(func() error {
				calls = append(calls, "features")
				if tc.failure == "features" {
					return errors.New("fixture build failure")
				}
				return nil
			}, func(args ...string) int {
				call := strings.Join(args, " ")
				calls = append(calls, call)
				if tc.failure == call {
					return 7
				}
				return 0
			})
			if code != tc.code || !reflect.DeepEqual(calls, tc.want) {
				t.Fatalf("code=%d calls=%v; want code=%d calls=%v", code, calls, tc.code, tc.want)
			}
		})
	}
}
