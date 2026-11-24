package chainrpc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Guard the production clients, not fixtures or unrelated HTTP integrations.
func TestCoreAndJobEthereumDialsUseSharedPolicy(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, dir := range []string{"pkg/common/chain", "pkg/core/service/project", "pkg/job/job/chainevent"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			for _, direct := range []string{"ethclient.Dial(", "ethclient.DialContext(", "rpc.DialHTTPWithClient(", "gethrpc.DialContext("} {
				if strings.Contains(string(data), direct) {
					t.Errorf("unguarded direct JSON-RPC client: %s", path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
