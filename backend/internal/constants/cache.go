package constants

// Redis cache keys shared by handlers and services.
const (
	// CacheKeyHomeOverview backs GET /home/overview (recommended pets etc.).
	// It must be invalidated whenever pet availability changes so that the
	// home page reflects newly (re-)available animals immediately.
	CacheKeyHomeOverview = "gbadopt:home:overview"
)
