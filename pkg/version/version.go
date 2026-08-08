package version

// These are stamped at build time via
// -ldflags "-X github.com/alswl/excalidraw-room-go/pkg/version.Version=..." (see
// hack/makefile-go/build.mk). Version defaults to "dev" for plain `go run`.
var (
	Version = "dev"
	Commit  = "unknown"
	Package = "github.com/alswl/excalidraw-room-go"
)
