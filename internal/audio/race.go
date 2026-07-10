//go:build race

package audio

// raceDetectorEnabled is true when built with -race. shine-mp3 uses unsafe
// pointer arithmetic that trips the -race checkptr instrumentation, so encoder
// tests are skipped under -race. Encoding is verified in non-race builds and
// works in production (non-race) binaries.
const raceDetectorEnabled = true
