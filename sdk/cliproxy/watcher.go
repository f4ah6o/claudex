package cliproxy

import (
	"context"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/watcher"
	coreauth "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func defaultWatcherFactory(configPath, authDir string, reload func(*config.Config)) (*WatcherWrapper, error) {
	return watcherFactoryWithLoader(configPath, authDir, reload, watcher.NewWatcher)
}

// NewWatcherFactoryWithConfigLoader creates a watcher factory that validates
// configuration updates with the supplied product-owned loader.
func NewWatcherFactoryWithConfigLoader(loader func(string) (*config.Config, error)) WatcherFactory {
	return func(configPath, authDir string, reload func(*config.Config)) (*WatcherWrapper, error) {
		return watcherFactoryWithLoader(configPath, authDir, reload, func(path, auth string, callback func(*config.Config)) (*watcher.Watcher, error) {
			return watcher.NewWatcherWithConfigLoader(path, auth, callback, loader)
		})
	}
}

func watcherFactoryWithLoader(configPath, authDir string, reload func(*config.Config), create func(string, string, func(*config.Config)) (*watcher.Watcher, error)) (*WatcherWrapper, error) {
	w, err := create(configPath, authDir, reload)
	if err != nil {
		return nil, err
	}

	return &WatcherWrapper{
		start: func(ctx context.Context) error {
			return w.Start(ctx)
		},
		stop: func() error {
			return w.Stop()
		},
		setConfig: func(cfg *config.Config) {
			w.SetConfig(cfg)
		},
		snapshotAuths: func() []*coreauth.Auth { return w.SnapshotCoreAuths() },
		setUpdateQueue: func(queue chan<- watcher.AuthUpdate) {
			w.SetAuthUpdateQueue(queue)
		},
		dispatchRuntimeUpdate: func(update watcher.AuthUpdate) bool {
			return w.DispatchRuntimeAuthUpdate(update)
		},
		dispatchPersistedAuth: func(update watcher.AuthUpdate) bool {
			return w.DispatchPersistedAuthUpdate(update)
		},
		setPluginAuthParser: func(parser PluginAuthParser) {
			w.SetPluginAuthParser(parser)
		},
		reloadConfigIfChanged: func() {
			w.ReloadConfigIfChanged()
		},
	}, nil
}
