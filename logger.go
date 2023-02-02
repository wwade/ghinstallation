package ghinstallation

type Logger interface {
	Printf(string, ...interface{})
}

type LeveledLogger interface {
	Infow(msg string, keysAndValues ...interface{})
	Debugw(msg string, keysAndValues ...interface{})
}
