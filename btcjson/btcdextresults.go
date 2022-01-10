package btcjson

//versionresult models object included in the version response ,in the actual
//result,these objects are keyed by the program or API name.

type VersionResult struct {
	VersionString string `json:"versionstring"`
	Major         uint32 `json:"major"`
	Path          uint32 `json:"path"`
	Prerelease    string `json:"prerelease"`
	BuildMetadata string `json:"build_metadata"`
}









