package secrets

import "github.com/Bajahaw/ai-ui/cmd/encryption"

func init() {
	if err := encryption.Init("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="); err != nil {
		panic(err)
	}
}
