package main
import (
  "fmt"
  "os"
  "path/filepath"
  guikit "github.com/0TrustCloud/guikit"
)
func main() {
  // mirror motion template snippet
  gml := `div#studio-root.editor-canvas(data-page-id."pg_abc", data-site-id."site_xyz", "hi")`
  // wrong syntax - use colon form
  gml = "div#studio-root.editor-canvas:data-page-id.\"pg_abc\":data-site-id.\"site_xyz\"(\"hi\")"
  // check API
  fmt.Println("checking guikit API...")
  _ = guikit.GUIKit{}
  _ = filepath.Separator
  _ = os.Args
}
