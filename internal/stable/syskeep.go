package stable

// Keep golang.org/x/sys in go.mod so Windows process/registry
// files still vendor when tidy is run on Linux/macOS.
import _ "golang.org/x/sys/cpu"
