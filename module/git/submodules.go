package git

import (
	// Submodules for the git app module registered here
	_ "github.com/vrongmeal/caddygit/services/poll"
	_ "github.com/vrongmeal/caddygit/services/webhook"
	_ "github.com/vrongmeal/caddygit/services/webhook/generic"
)
