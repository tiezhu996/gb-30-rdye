package constants

// Redis cache keys and related configuration.
const (
	// CacheKeyHomeOverview backs GET /home/overview (hot pets / posts / orgs).
	// It must be invalidated whenever pet availability changes so that
	// re-adoptable animals show up on the home page immediately.
	CacheKeyHomeOverview = "gbadopt:home:overview"
)
