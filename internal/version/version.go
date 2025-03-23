package version

// Version is the version of the application
var Version = "dev"

// Commit is the git commit SHA of the application
var Commit = "unknown"

// BuildTime is the time the application was built
var BuildTime = "unknown"

// GetVersion returns the full version information
func GetVersion() map[string]string {
	return map[string]string{
		"version":   Version,
		"commit":    Commit,
		"buildTime": BuildTime,
	}
}
