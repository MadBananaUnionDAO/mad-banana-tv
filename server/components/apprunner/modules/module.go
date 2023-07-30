package modules

import (
	"context"

	"github.com/dop251/goja_nodejs/require"
)

// NativeModule is a module that can be imported into a single application instance
type NativeModule interface {
	ModuleLoader() require.ModuleLoader
	ModuleName() string
	IsNodeBuiltin() bool
	AutoRequire() (bool, string)
	ExecutionResumed(context.Context)
	ExecutionPaused()
}

// ApplicationLogger logs application actions
type ApplicationLogger interface {
	RuntimeAuditLog(s string)
}
