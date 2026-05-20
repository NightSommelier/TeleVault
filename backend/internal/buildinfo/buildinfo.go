package buildinfo

const defaultVersion = "0.1.0-dev"

var (
	Version = defaultVersion
	Commit  = "unknown"
	Date    = "unknown"
)

func Info() map[string]string {
	return map[string]string{
		"version": Version,
		"commit":  Commit,
		"date":    Date,
	}
}
