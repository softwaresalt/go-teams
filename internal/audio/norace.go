//go:build !race

package audio

// raceDetectorEnabled is false in normal (non-race) builds.
const raceDetectorEnabled = false
