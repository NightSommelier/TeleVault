package buildinfo

var (
	Version = generatedVersion
	Commit  = generatedCommit
	Date    = generatedDate
)

func Info() map[string]string {
	return map[string]string{
		"version": Version,
		"commit":  Commit,
		"date":    Date,
	}
}
