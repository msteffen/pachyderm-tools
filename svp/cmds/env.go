package cmds

var Env struct {
	// Config is a struct containing all fields defined in the .svpconfig file
	// (this is how configured values can be accessed)
	Config struct {
		ClientDirectory string // The top-level directory containing all clients
		DiffTool        string // The user's preferred tool for diffing branches
	}

	// Information about this client's git repo
	CurBranch string
	GitRoot   string

	// A map from client name (e.g. "pfs-v2") to information about the client
	// (e.g. whether there's an open pull request for it)
	// This is cached
	Clients map[string]struct {
		IsPullRequestOpen *bool // Can be true or false, or unset if unknown
	}

	// If true, then some cached field has been updated, and the cache file needs
	// to be re-written
	IsCacheStale bool
}

// func init() error {
// 	getCachedClientInfo()
// 	getClientInfoSlow()
// }

// func initCache() {
// }
