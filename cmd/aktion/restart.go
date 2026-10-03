package main

// Complete all builds while the old listeners are still running. Once down has
// succeeded, up must use those built images without another fallible build.
func restartStack(buildFeatures func() error, compose func(...string) int) int {
	if err := buildFeatures(); err != nil {
		return fail(err)
	}
	if code := compose("build"); code != 0 {
		return code
	}
	if code := compose("down"); code != 0 {
		return code
	}
	return compose("up", "-d", "--no-build")
}
