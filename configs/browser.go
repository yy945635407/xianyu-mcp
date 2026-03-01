package configs

var (
	useHeadless = true
	binPath     = ""
)

func InitHeadless(h bool) {
	useHeadless = h
}

func IsHeadless() bool {
	return useHeadless
}

func SetBinPath(b string) {
	binPath = b
}

func GetBinPath() string {
	return binPath
}
