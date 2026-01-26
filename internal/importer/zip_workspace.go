package importer

import "os"

type ZipWorkspace struct {
	Root string
}

func (ws *ZipWorkspace) Cleanup() error {
	if ws == nil || ws.Root == "" {
		return nil
	}
	return os.RemoveAll(ws.Root)
}
