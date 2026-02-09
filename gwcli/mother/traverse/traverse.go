// TODO annotate
package traverse

import "github.com/spf13/cobra"

// Up return the parent directory to the given command.
// Returns itself if it has no parent.
func Up(dir *cobra.Command) *cobra.Command {
	if dir.Parent() == nil { // if we are at root, do nothing
		return dir
	}
	// otherwise, step upward
	return dir.Parent()
}

func IsRootTraversalToken() bool {
	// TODO
}

func IsUpTraversalToken() bool {
	// TODO
}
